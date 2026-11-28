package book

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"
)

var (
	errZoteroLocalAPIDisabled    = errors.New("Zotero local API is disabled")
	errZoteroLocalAPIUnavailable = errors.New("Zotero local API is unavailable")
)

const zoteroLocalLibraryURL = "http://127.0.0.1:23119/api/users/0"

type zoteroLocalAPI struct {
	client *http.Client
}

func newZoteroLocalAPI() zoteroLocalAPI {
	return zoteroLocalAPI{client: &http.Client{
		// Connect directly to Zotero and inspect file redirects without following them.
		Transport: &http.Transport{DisableKeepAlives: true},
		Timeout:   4 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}}
}

type zoteroAPICollection struct {
	Data struct {
		Key              string `json:"key"`
		Name             string `json:"name"`
		ParentCollection any    `json:"parentCollection"`
	} `json:"data"`
}

type zoteroAPIItem struct {
	Data struct {
		Key         string          `json:"key"`
		ItemType    string          `json:"itemType"`
		ParentItem  any             `json:"parentItem"`
		Title       string          `json:"title"`
		Collections []string        `json:"collections"`
		LinkMode    string          `json:"linkMode"`
		ContentType string          `json:"contentType"`
		Filename    string          `json:"filename"`
		Deleted     json.RawMessage `json:"deleted"`
	} `json:"data"`
	Links struct {
		Enclosure struct {
			Href any `json:"href"`
		} `json:"enclosure"`
	} `json:"links"`
}

func (api zoteroLocalAPI) readLibrary(root string) (ZoteroLibrary, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	collections, err := listZoteroAPI[zoteroAPICollection](ctx, api, "/collections")
	if err != nil {
		return ZoteroLibrary{}, err
	}
	items, err := listZoteroAPI[zoteroAPIItem](ctx, api, "/items")
	if err != nil {
		return ZoteroLibrary{}, err
	}

	library := ZoteroLibrary{Collections: []ZoteroCollection{}, PDFs: []ZoteroPDF{}}
	// Keep IDs stable across requests and distinct from positive SQLite IDs.
	ids := make(map[string]int64, len(collections))
	for _, collection := range collections {
		// Zotero collection keys fit in 52 bits when decoded as base 36.
		id, err := strconv.ParseUint(collection.Data.Key, 36, 52)
		if err != nil || id == 0 {
			return ZoteroLibrary{}, fmt.Errorf("Zotero 本地 API 返回了无效的集合 ID")
		}
		ids[collection.Data.Key] = -int64(id)
	}
	for _, collection := range collections {
		data := collection.Data
		parentKey, _ := data.ParentCollection.(string) // Zotero uses false for root collections.
		library.Collections = append(library.Collections, ZoteroCollection{
			ID: ids[data.Key], ParentID: ids[parentKey], Name: data.Name,
		})
	}
	slices.SortStableFunc(library.Collections, func(a, b ZoteroCollection) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})

	itemsByKey := make(map[string]zoteroAPIItem, len(items))
	for _, item := range items {
		itemsByKey[item.Data.Key] = item
	}
	for _, item := range items {
		data := item.Data
		if zoteroItemDeleted(data.Deleted) || data.ItemType != "attachment" ||
			(data.LinkMode != "imported_file" && data.LinkMode != "imported_url") ||
			(data.ContentType != "application/pdf" && !strings.EqualFold(filepath.Ext(data.Filename), ".pdf")) {
			continue
		}
		owner := item
		if parent, ok := data.ParentItem.(string); ok && parent != "" {
			var ok bool
			owner, ok = itemsByKey[parent]
			if !ok || zoteroItemDeleted(owner.Data.Deleted) {
				continue
			}
		}

		path, err := api.attachmentPath(ctx, root, data.Key, data.Filename, item.Links.Enclosure.Href)
		if err != nil {
			return ZoteroLibrary{}, err
		}
		if path == "" {
			library.Missing++
			continue
		}
		title := owner.Data.Title
		if strings.TrimSpace(title) == "" {
			title = data.Filename
		}
		pdf := ZoteroPDF{
			Book:     BookInfo{ID: path, Path: path, Title: title},
			Filename: data.Filename, CollectionIDs: []int64{},
		}
		for _, key := range owner.Data.Collections {
			if id, ok := ids[key]; ok {
				pdf.CollectionIDs = append(pdf.CollectionIDs, id)
			}
		}
		library.PDFs = append(library.PDFs, pdf)
	}
	return library, nil
}

func zoteroItemDeleted(value json.RawMessage) bool {
	return string(value) == "1" || string(value) == "true"
}

func listZoteroAPI[T any](ctx context.Context, api zoteroLocalAPI, endpoint string) ([]T, error) {
	const pageSize = 100
	var results []T
	for start := 0; ; {
		response, err := api.get(ctx, fmt.Sprintf("%s?format=json&limit=%d&start=%d", endpoint, pageSize, start))
		if err != nil {
			return nil, err
		}
		var page []T
		status := response.StatusCode
		total, totalErr := strconv.Atoi(response.Header.Get("Total-Results"))
		next := strings.Contains(response.Header.Get("Link"), `rel="next"`)
		if status == http.StatusOK {
			err = json.NewDecoder(io.LimitReader(response.Body, 32<<20)).Decode(&page)
		}
		response.Body.Close()
		if status != http.StatusOK {
			return nil, zoteroAPIStatus(status)
		}
		if err != nil {
			return nil, fmt.Errorf("Zotero 本地 API 返回的数据无法解析：%w", err)
		}
		results = append(results, page...)
		start += len(page)
		if len(page) == 0 || (!next && (totalErr == nil && start >= total || totalErr != nil && len(page) < pageSize)) {
			return results, nil
		}
	}
}

func (api zoteroLocalAPI) attachmentPath(ctx context.Context, root, key, filename string, enclosure any) (string, error) {
	if !zoteroPathSegment(key) || !zoteroPathSegment(filename) {
		return "", nil
	}
	href, _ := enclosure.(string)
	if enclosure == false {
		return "", nil
	}
	if href == "" {
		response, err := api.get(ctx, "/items/"+url.PathEscape(key)+"/file")
		if err != nil {
			return "", err
		}
		defer response.Body.Close()
		switch response.StatusCode {
		case http.StatusNotFound:
			return "", nil
		case http.StatusFound:
			href = response.Header.Get("Location")
			if href == "" || href == "false" {
				return "", nil
			}
		default:
			return "", zoteroAPIStatus(response.StatusCode)
		}
	}
	location, err := url.Parse(href)
	if err != nil || location.Scheme != "file" || (location.Host != "" && location.Host != "localhost") {
		return "", fmt.Errorf("Zotero 本地 API 返回了无效的附件路径")
	}
	localPath := location.Path
	if runtime.GOOS == "windows" && len(localPath) >= 3 && localPath[0] == '/' && localPath[2] == ':' {
		localPath = localPath[1:]
	}
	expected := filepath.Join(root, "storage", key, filename)
	expectedDir := filepath.Dir(expected)
	actualDir := filepath.Dir(filepath.FromSlash(localPath))
	// Compare directories after resolving aliases such as macOS /tmp.
	if resolved, err := filepath.EvalSymlinks(expectedDir); err == nil {
		expectedDir = resolved
	}
	if resolved, err := filepath.EvalSymlinks(actualDir); err == nil {
		actualDir = resolved
	}
	matches := expectedDir == actualDir && filename == filepath.Base(localPath)
	if runtime.GOOS == "windows" {
		matches = strings.EqualFold(expectedDir, actualDir) && strings.EqualFold(filename, filepath.Base(localPath))
	}
	if !matches {
		return "", fmt.Errorf("正在运行的 Zotero 使用了其他数据目录，请选择 Zotero 设置中显示的数据目录")
	}
	info, err := os.Stat(expected)
	if err != nil || !info.Mode().IsRegular() {
		return "", nil
	}
	return expected, nil
}

func (api zoteroLocalAPI) get(ctx context.Context, endpoint string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, zoteroLocalLibraryURL+endpoint, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Zotero-API-Version", "3")
	response, err := api.client.Do(request)
	if err != nil {
		var networkErr *net.OpError
		if errors.As(err, &networkErr) || errors.Is(err, context.DeadlineExceeded) {
			return nil, errZoteroLocalAPIUnavailable
		}
		return nil, fmt.Errorf("Zotero 本地 API 请求失败：%w", err)
	}
	return response, nil
}

func zoteroAPIStatus(status int) error {
	if status == http.StatusForbidden {
		return errZoteroLocalAPIDisabled
	}
	return fmt.Errorf("数据库已锁定，Zotero 本地 API 返回 HTTP %d", status)
}

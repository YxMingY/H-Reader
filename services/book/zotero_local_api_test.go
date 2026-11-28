package book

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type zoteroTestTransport func(*http.Request) (*http.Response, error)

func (f zoteroTestTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestLockedZoteroUsesLocalAPI(t *testing.T) {
	root := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(root, "zotero.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("CREATE TABLE fixture (id INT); BEGIN EXCLUSIVE"); err != nil {
		t.Fatal(err)
	}
	defer db.Exec("ROLLBACK")

	path := filepath.Join(root, "storage", "FILE0001", "Paper 测试.pdf")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("%PDF-1.4"), 0600); err != nil {
		t.Fatal(err)
	}
	filePath := filepath.ToSlash(path)
	if !strings.HasPrefix(filePath, "/") {
		filePath = "/" + filePath
	}
	fileURL := (&url.URL{Scheme: "file", Path: filePath}).String()
	encodedURL, err := json.Marshal(fileURL)
	if err != nil {
		t.Fatal(err)
	}

	api := newZoteroLocalAPI()
	api.client.Transport = zoteroTestTransport(func(request *http.Request) (*http.Response, error) {
		if request.Method != "GET" || request.URL.Host != "127.0.0.1:23119" || request.Header.Get("Zotero-API-Version") != "3" {
			t.Fatalf("unexpected request: %v", request)
		}
		response := &http.Response{StatusCode: 200, Header: make(http.Header)}
		var body string
		switch request.URL.Path {
		case "/api/users/0/collections":
			body = `[{"data":{"key":"AAAA0001","name":"Parent","parentCollection":false}},
				{"data":{"key":"BBBB0001","name":"Child","parentCollection":"AAAA0001"}}]`
		case "/api/users/0/items":
			response.Header.Set("Total-Results", "4")
			if request.URL.Query().Get("start") == "0" {
				body = `[{"data":{"key":"ITEM0001","itemType":"book","title":"Parent title","collections":["AAAA0001","BBBB0001"]}}]`
			} else if request.URL.Query().Get("start") == "1" {
				body = `[{"data":{"key":"FILE0001","itemType":"attachment","linkMode":"imported_file","parentItem":"ITEM0001",
					"contentType":"application/pdf","filename":"Paper 测试.pdf"},"links":{"enclosure":{"href":` + string(encodedURL) + `}}},
					{"data":{"key":"FILE0002","itemType":"attachment","linkMode":"imported_file","parentItem":false,"contentType":"application/pdf","filename":"Missing.pdf"}},
					{"data":{"key":"FILE0003","itemType":"attachment","linkMode":"imported_file","contentType":"application/pdf","deleted":1,"filename":"Deleted.pdf"}}]`
			} else {
				t.Fatalf("unexpected pagination: %s", request.URL)
			}
		case "/api/users/0/items/FILE0002/file":
			response.StatusCode = 404
		default:
			t.Fatalf("unexpected endpoint: %s", request.URL)
		}
		response.Body = io.NopCloser(strings.NewReader(body))
		return response, nil
	})

	library, err := readZoteroLibrary(root, api)
	if err != nil {
		t.Fatal(err)
	}
	if library.Missing != 1 || len(library.PDFs) != 1 || len(library.Collections) != 2 {
		t.Fatalf("unexpected library: %+v", library)
	}
	child, parent := library.Collections[0], library.Collections[1]
	if child.ParentID != parent.ID || child.Name != "Child" || parent.Name != "Parent" {
		t.Fatalf("unexpected collections: %+v", library.Collections)
	}
	pdf := library.PDFs[0]
	if pdf.Book.Path != path || pdf.Book.Title != "Parent title" || len(pdf.CollectionIDs) != 2 ||
		pdf.CollectionIDs[0] != parent.ID || pdf.CollectionIDs[1] != child.ID {
		t.Fatalf("unexpected PDF: %+v", pdf)
	}
	if _, err := api.attachmentPath(t.Context(), t.TempDir(), "FILE0001", pdf.Filename, fileURL); err == nil {
		t.Fatal("accepted an attachment from another data directory")
	}
}

func TestLockedZoteroReturnsSetupStatus(t *testing.T) {
	root := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(root, "zotero.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("CREATE TABLE fixture (id INT); BEGIN EXCLUSIVE"); err != nil {
		t.Fatal(err)
	}
	defer db.Exec("ROLLBACK")

	for _, example := range []struct {
		name         string
		status       int
		transportErr error
		wantSetup    string
		wantError    string
	}{
		{name: "disabled", status: 403, wantSetup: "disabled"},
		{name: "unavailable", transportErr: &net.OpError{Op: "dial", Err: errors.New("connection refused")}, wantSetup: "unavailable"},
		{name: "unexpected HTTP failure", status: 500, wantError: "HTTP 500"},
	} {
		t.Run(example.name, func(t *testing.T) {
			api := newZoteroLocalAPI()
			api.client.Transport = zoteroTestTransport(func(*http.Request) (*http.Response, error) {
				if example.transportErr != nil {
					return nil, example.transportErr
				}
				return &http.Response{StatusCode: example.status, Body: http.NoBody}, nil
			})
			library, err := readZoteroLibrary(root, api)
			if example.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), example.wantError) {
					t.Fatalf("want %q, got %v", example.wantError, err)
				}
				return
			}
			if err != nil || library.LocalAPISetup != example.wantSetup {
				t.Fatalf("want setup status %q, got %+v, %v", example.wantSetup, library, err)
			}
		})
	}
}

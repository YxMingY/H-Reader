package book

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

type ZoteroCollection struct {
	ID       int64  `json:"id"`
	ParentID int64  `json:"parentId"`
	Name     string `json:"name"`
}

type ZoteroPDF struct {
	Book          BookInfo `json:"book"`
	Filename      string   `json:"filename"`
	CollectionIDs []int64  `json:"collectionIds"`
}

type ZoteroLibrary struct {
	Collections   []ZoteroCollection `json:"collections"`
	PDFs          []ZoteroPDF        `json:"pdfs"`
	Missing       int                `json:"missing"`
	LocalAPISetup string             `json:"localApiSetup,omitempty"`
}

func (a *BookService) HasZoteroLibrary(dir string) bool {
	if dir == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(dir, "zotero.sqlite"))
	// An unreadable database should still surface a useful error when opened.
	return os.IsPermission(err) || (err == nil && info.Mode().IsRegular())
}

func (a *BookService) ReadZoteroLibrary(dir string) (ZoteroLibrary, error) {
	return readZoteroLibrary(dir, newZoteroLocalAPI())
}

func readZoteroLibrary(dir string, api zoteroLocalAPI) (ZoteroLibrary, error) {
	root, err := filepath.Abs(dir)
	if err != nil {
		return ZoteroLibrary{}, err
	}
	library, err := readZoteroSQLite(root)
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		switch sqliteErr.Code() & 0xff {
		case sqlite3.SQLITE_BUSY, sqlite3.SQLITE_LOCKED:
			library, err = api.readLibrary(root)
			switch {
			case errors.Is(err, errZoteroLocalAPIDisabled):
				return ZoteroLibrary{LocalAPISetup: "disabled"}, nil
			case errors.Is(err, errZoteroLocalAPIUnavailable):
				return ZoteroLibrary{LocalAPISetup: "unavailable"}, nil
			}
		}
	}
	if err != nil {
		return ZoteroLibrary{}, fmt.Errorf("无法读取 Zotero 文库：%w", err)
	}
	return library, nil
}

func readZoteroSQLite(root string) (ZoteroLibrary, error) {
	var library ZoteroLibrary
	databasePath := filepath.ToSlash(filepath.Join(root, "zotero.sqlite"))
	if !strings.HasPrefix(databasePath, "/") {
		databasePath = "/" + databasePath
	}
	dsn := url.URL{Scheme: "file", Path: databasePath, RawQuery: "mode=ro&_pragma=query_only(1)&_pragma=busy_timeout(500)"}
	db, err := sql.Open("sqlite", dsn.String())
	if err != nil {
		return library, err
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return library, err
	}
	defer tx.Rollback()

	collections, err := readZoteroCollections(ctx, tx)
	if err != nil {
		return library, err
	}
	pdfs, missing, err := readZoteroPDFs(ctx, tx, root)
	if err != nil {
		return library, err
	}
	return ZoteroLibrary{Collections: collections, PDFs: pdfs, Missing: missing}, nil
}

func readZoteroCollections(ctx context.Context, tx *sql.Tx) ([]ZoteroCollection, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT c.collectionID, COALESCE(c.parentCollectionID, 0), c.collectionName
		FROM collections c JOIN libraries l USING (libraryID)
		WHERE l.type = 'user'
			AND NOT EXISTS (SELECT 1 FROM deletedCollections x WHERE x.collectionID = c.collectionID)
		ORDER BY c.collectionName COLLATE NOCASE, c.collectionID`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	collections := []ZoteroCollection{}
	for rows.Next() {
		var collection ZoteroCollection
		if err := rows.Scan(&collection.ID, &collection.ParentID, &collection.Name); err != nil {
			return nil, err
		}
		collections = append(collections, collection)
	}
	return collections, rows.Err()
}

func readZoteroPDFs(ctx context.Context, tx *sql.Tx, root string) ([]ZoteroPDF, int, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT i.itemID, i.key, a.path, COALESCE(title.value, ''), COALESCE(c.collectionID, 0)
		FROM itemAttachments a
		JOIN items i USING (itemID)
		JOIN libraries l USING (libraryID)
		LEFT JOIN itemData d ON d.itemID = COALESCE(a.parentItemID, i.itemID)
			AND d.fieldID = (SELECT fieldID FROM fieldsCombined WHERE fieldName = 'title')
		LEFT JOIN itemDataValues title USING (valueID)
		LEFT JOIN collectionItems ci ON ci.itemID = COALESCE(a.parentItemID, i.itemID)
		LEFT JOIN collections c ON c.collectionID = ci.collectionID AND c.libraryID = i.libraryID
		WHERE l.type = 'user' AND a.linkMode IN (0, 1)
			AND a.path LIKE 'storage:%'
			AND (a.contentType = 'application/pdf' OR a.path LIKE '%.pdf')
			AND NOT EXISTS (SELECT 1 FROM deletedItems x WHERE x.itemID IN (i.itemID, a.parentItemID))
		ORDER BY i.itemID, c.collectionID`)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	pdfs := []ZoteroPDF{}
	missing := 0
	var previousID int64
	var pdf *ZoteroPDF
	for rows.Next() {
		var id, collectionID int64
		var key, storedPath, title string
		if err := rows.Scan(&id, &key, &storedPath, &title, &collectionID); err != nil {
			return nil, 0, err
		}
		if id != previousID {
			previousID, pdf = id, nil
			filename := strings.TrimPrefix(storedPath, "storage:")
			if !zoteroPathSegment(key) || !zoteroPathSegment(filename) {
				missing++
				continue
			}
			path := filepath.Join(root, "storage", key, filename)
			info, err := os.Stat(path)
			if err != nil || !info.Mode().IsRegular() {
				missing++
				continue
			}
			if strings.TrimSpace(title) == "" {
				title = filename
			}
			pdfs = append(pdfs, ZoteroPDF{
				Book:     BookInfo{ID: path, Path: path, Title: title},
				Filename: filename, CollectionIDs: []int64{},
			})
			pdf = &pdfs[len(pdfs)-1]
		}
		if pdf != nil && collectionID != 0 {
			pdf.CollectionIDs = append(pdf.CollectionIDs, collectionID)
		}
	}
	return pdfs, missing, rows.Err()
}

func zoteroPathSegment(value string) bool {
	return filepath.IsLocal(value) && value != "." && filepath.Base(value) == value && !strings.ContainsRune(value, ':')
}

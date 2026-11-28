package book

import (
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadZoteroLibrary(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, "zotero.sqlite")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		CREATE TABLE libraries (libraryID INTEGER PRIMARY KEY, type TEXT);
		CREATE TABLE collections (collectionID INTEGER PRIMARY KEY, parentCollectionID INTEGER, collectionName TEXT, libraryID INTEGER);
		CREATE TABLE items (itemID INTEGER PRIMARY KEY, key TEXT, libraryID INTEGER);
		CREATE TABLE itemAttachments (itemID INTEGER PRIMARY KEY, parentItemID INTEGER, path TEXT, linkMode INTEGER, contentType TEXT);
		CREATE TABLE collectionItems (collectionID INTEGER, itemID INTEGER);
		CREATE TABLE itemData (itemID INTEGER, fieldID INTEGER, valueID INTEGER);
		CREATE TABLE itemDataValues (valueID INTEGER, value TEXT);
		CREATE TABLE fieldsCombined (fieldID INTEGER, fieldName TEXT);
		CREATE TABLE deletedCollections (collectionID INTEGER);
		CREATE TABLE deletedItems (itemID INTEGER);
		INSERT INTO libraries VALUES (1, 'user'), (2, 'group');
		INSERT INTO collections VALUES (1, NULL, 'Papers', 1), (2, 1, 'Papers', 1), (3, NULL, 'Group', 2);
		INSERT INTO items VALUES (10, 'PARENT00', 1), (11, 'ATTACH01', 1), (12, 'ATTACH02', 1),
			(13, 'ATTACH03', 1), (14, 'ATTACH04', 1), (15, 'ATTACH05', 2), (16, 'PARENT01', 1), (17, 'ATTACH07', 1);
		INSERT INTO itemAttachments VALUES
			(11, 10, 'storage:Paper.pdf', 0, 'application/pdf'),
			(12, NULL, 'storage:Standalone.PDF', 1, 'application/octet-stream'),
			(13, 10, 'storage:Missing.pdf', 0, 'application/pdf'),
			(14, NULL, 'storage:Deleted.pdf', 0, 'application/pdf'),
			(15, NULL, 'storage:Group.pdf', 0, 'application/pdf'),
			(17, 16, 'storage:Deleted-parent.pdf', 0, 'application/pdf');
		INSERT INTO collectionItems VALUES (1, 10), (2, 10);
		INSERT INTO itemDataValues VALUES (1, 'Zotero title');
		INSERT INTO itemData VALUES (10, 1, 1);
		INSERT INTO fieldsCombined VALUES (1, 'title');
		INSERT INTO deletedItems VALUES (14), (16);
	`)
	closeErr := db.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}

	for key, filename := range map[string]string{"ATTACH01": "Paper.pdf", "ATTACH02": "Standalone.PDF"} {
		dir := filepath.Join(root, "storage", key)
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, filename), []byte("%PDF-1.4"), 0600); err != nil {
			t.Fatal(err)
		}
	}

	service := &BookService{}
	if !service.HasZoteroLibrary(root) || service.HasZoteroLibrary(t.TempDir()) {
		t.Fatal("Zotero directory detection failed")
	}
	library, err := service.ReadZoteroLibrary(root)
	if err != nil {
		t.Fatal(err)
	}
	expectedCollections := []ZoteroCollection{{ID: 1, Name: "Papers"}, {ID: 2, ParentID: 1, Name: "Papers"}}
	if !reflect.DeepEqual(library.Collections, expectedCollections) {
		t.Fatalf("unexpected collections: %+v", library.Collections)
	}
	if len(library.PDFs) != 2 || library.Missing != 1 {
		t.Fatalf("want two local PDFs and one missing file: %+v", library)
	}
	paper := library.PDFs[0]
	if paper.Book.Title != "Zotero title" || paper.Filename != "Paper.pdf" || !reflect.DeepEqual(paper.CollectionIDs, []int64{1, 2}) {
		t.Fatalf("unexpected collection PDF: %+v", paper)
	}
	standalone := library.PDFs[1]
	if standalone.Book.Title != "Standalone.PDF" || len(standalone.CollectionIDs) != 0 {
		t.Fatalf("unexpected unfiled PDF: %+v", standalone)
	}
}

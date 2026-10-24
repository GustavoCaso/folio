package handlers_test

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/GustavoCaso/folio/ui/internal/domain"
)

func TestEditDocument_NormalizesAndDedupesTags(t *testing.T) {
	store := newTestStore(t)
	mux, _ := newTestMux(t, store)

	document, err := store.CreateDocument(context.Background(), "dune.epub", []byte("x"), "req", domain.EpubFormat)
	if err != nil {
		t.Fatal(err)
	}

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	_ = mw.WriteField("title", "Dune")
	_ = mw.WriteField("author", "Frank Herbert")
	_ = mw.WriteField("tags", " Sci-Fi, sci-fi ,Classic,, classic ,SCI-FI")
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/documents/"+document.ID+"/edit", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusSeeOther)
	}
	got, err := store.GetDocument(context.Background(), document.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"sci-fi", "classic"}
	if !reflect.DeepEqual(got.Tags, want) {
		t.Errorf("tags: got %v, want %v", got.Tags, want)
	}
}

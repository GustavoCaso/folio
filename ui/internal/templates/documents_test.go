package templates_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/GustavoCaso/folio/ui/internal/db"
	"github.com/GustavoCaso/folio/ui/internal/templates"
)

func renderDocuments(t *testing.T, jobs []db.Job, watchJobIDs ...string) string {
	t.Helper()
	var buf bytes.Buffer
	if err := templates.Documents(jobs, watchJobIDs, "").Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

func renderDocumentList(t *testing.T, jobs []db.Job) string {
	t.Helper()
	var buf bytes.Buffer
	if err := templates.DocumentList(jobs).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

func TestDocumentList_RendersIDAndFilename(t *testing.T) {
	jobs := []db.Job{
		{ID: "j1", Filename: "alpha.pdf", Status: "PENDING"},
		{ID: "j2", Filename: "beta.pdf", Status: "PENDING"},
	}
	got := renderDocumentList(t, jobs)

	for _, want := range []string{`id="job-j1"`, "alpha.pdf", `id="job-j2"`, "beta.pdf"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output, got: %s", want, got)
		}
	}
}

func TestDocumentList_ProcessingOmitsPageCountWhenZero(t *testing.T) {
	got := renderDocumentList(t, []db.Job{
		{ID: "j1", Filename: "doc.pdf", Status: "PROCESSING", PagesDone: 0, PagesTotal: 0},
	})

	if strings.Contains(got, "pages") {
		t.Errorf("expected no page count when PagesTotal is 0, got: %s", got)
	}
}

func TestDocumentList_DoneShowsReadLink(t *testing.T) {
	got := renderDocumentList(t, []db.Job{
		{ID: "j2", Filename: "beta.pdf", Status: "DONE"},
	})

	if !strings.Contains(got, `href="/read/j2"`) {
		t.Errorf("expected read link in output, got: %s", got)
	}
}

func TestDocumentList_FailedShowsErrorMessage(t *testing.T) {
	got := renderDocumentList(t, []db.Job{
		{ID: "j3", Filename: "bad.pdf", Status: "FAILED", Error: "parser timeout"},
	})

	if !strings.Contains(got, "parser timeout") {
		t.Errorf("expected error message in output, got: %s", got)
	}
}

func TestDocumentList_EmptyRendersNothing(t *testing.T) {
	got := renderDocumentList(t, nil)

	if strings.Contains(got, "<ul>") {
		t.Errorf("expected no <ul> for empty job list, got: %s", got)
	}
}

func TestDocumentsEmbedsJobIDInDataAttribute(t *testing.T) {
	got := renderDocuments(t, nil, "job-abc-123")

	if !strings.Contains(got, `data-job-ids="job-abc-123"`) {
		t.Errorf("expected data-job-ids attribute with job id, got: %s", got)
	}
}

func TestDocumentsScriptHasNoUnrenderedTemplExpression(t *testing.T) {
	got := renderDocuments(t, nil, "job-1")

	// Regression: templ does NOT interpolate { ... } inside raw <script>.
	// Previous code shipped `const jobID = { templ.JSONString(watchJobID) };`
	// to the browser, causing a JS SyntaxError on the `.`.
	if strings.Contains(got, "templ.JSONString") {
		t.Errorf("script contains unrendered templ expression: %s", got)
	}
}

func TestDocumentsOmitsScriptWhenNoWatchJobID(t *testing.T) {
	got := renderDocuments(t, nil)

	if strings.Contains(got, "EventSource") {
		t.Errorf("expected no EventSource script when watchJobIDs empty, got: %s", got)
	}
}

func TestDocumentsIncludesDocumentList(t *testing.T) {
	jobs := []db.Job{
		{ID: "j1", Filename: "alpha.pdf", Status: "DONE"},
	}
	got := renderDocuments(t, jobs)

	if !strings.Contains(got, `id="job-j1"`) {
		t.Errorf("expected DocumentList output inside Documents, got: %s", got)
	}
}

func TestDocumentsShowsErrorBanner(t *testing.T) {
	var buf bytes.Buffer
	if err := templates.Documents(nil, nil, "Something went wrong").Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	got := buf.String()

	if !strings.Contains(got, `class="error-banner"`) {
		t.Errorf("expected error-banner element in output, got: %s", got)
	}
	if !strings.Contains(got, "Something went wrong") {
		t.Errorf("expected error message in output, got: %s", got)
	}
}

func TestDocumentsOmitsErrorBannerWhenNoError(t *testing.T) {
	got := renderDocuments(t, nil)

	if strings.Contains(got, `class="error-banner"`) {
		t.Errorf("expected no error-banner element when errMsg empty, got: %s", got)
	}
}

func TestDocumentsWatcherScriptMatchesExpected(t *testing.T) {
	got := renderDocuments(t, nil, "job-1")

	want := `<script>
				(function() {
					function injectDeleteForm(li, jobID) {
						const slot = li.querySelector(".delete-action");
						if (!slot || slot.querySelector("form")) return;
						const filename = li.querySelector(".filename").textContent;
						const form = document.createElement("form");
						form.method = "POST";
						form.action = "/documents/" + jobID + "/delete";
						form.dataset.filename = filename;
						const btn = document.createElement("button");
						btn.type = "submit";
						btn.textContent = "Delete";
						form.appendChild(btn);
						form.addEventListener("submit", function(e) {
							if (!confirm("Delete « " + filename + " »?")) e.preventDefault();
						});
						slot.appendChild(form);
					}
					const ids = document.getElementById("watch-config").dataset.jobIds.split(",");
					ids.forEach(function(jobID) {
						const li = document.getElementById("job-" + jobID);
						if (!li) return;
						const es = new EventSource("/jobs/" + jobID + "/watch");
						es.addEventListener("status", function(e) {
							const d = JSON.parse(e.data);
							if (d.Status) {
								const status = li.querySelector(".status");
								status.textContent = d.Status;
								status.className = "status status-" + d.Status;
							}

							const detail = li.querySelector(".detail");
							if (d.Status === "DONE") {
								detail.textContent = "";
								const link = li.querySelector(".read-link");
								link.innerHTML = '— <a href="/read/' + jobID + '">Read</a>';
								injectDeleteForm(li, jobID);
								es.close();
							} else if (d.Status === "FAILED") {
								detail.textContent = d.Error || "";
								injectDeleteForm(li, jobID);
								es.close();
							} else if (d.Stage) {
								let text = d.Stage;
								if (d.Message) text += " — " + d.Message;
								if (d.PagesTotal) text += " (" + d.PagesDone + "/" + d.PagesTotal + ")";
								detail.textContent = text;
							}
						});
					});
				})();
			</script>`

	if !strings.Contains(got, want) {
		t.Errorf("watcher script does not match expected.\nwant:\n%s\n\ngot:\n%s", want, got)
	}
}

func TestDocumentList_ShowsDeleteForDone(t *testing.T) {
	body := renderDocumentList(t, []db.Job{
		{ID: "abc123", Filename: "done.pdf", Status: "DONE"},
	})
	if !strings.Contains(body, `/documents/abc123/delete`) {
		t.Errorf("expected delete link for DONE job, got:\n%s", body)
	}
}

func TestDocumentList_ShowsDeleteForFailed(t *testing.T) {
	body := renderDocumentList(t, []db.Job{
		{ID: "def456", Filename: "failed.pdf", Status: "FAILED", Error: "parse error"},
	})
	if !strings.Contains(body, `/documents/def456/delete`) {
		t.Errorf("expected delete link for FAILED job, got:\n%s", body)
	}
}

func TestDocumentList_NoDeleteForPending(t *testing.T) {
	body := renderDocumentList(t, []db.Job{
		{ID: "ghi789", Filename: "pending.pdf", Status: "PENDING"},
	})
	if strings.Contains(body, `/documents/ghi789/delete`) {
		t.Errorf("expected no delete link for PENDING job, got:\n%s", body)
	}
}

func TestDocumentList_NoDeleteForProcessing(t *testing.T) {
	body := renderDocumentList(t, []db.Job{
		{ID: "jkl012", Filename: "processing.pdf", Status: "PROCESSING"},
	})
	if strings.Contains(body, `/documents/jkl012/delete`) {
		t.Errorf("expected no delete link for PROCESSING job, got:\n%s", body)
	}
}

func TestDocumentsEmbedsMultipleJobIDs(t *testing.T) {
	got := renderDocuments(t, nil, "job-1", "job-2")

	if !strings.Contains(got, `data-job-ids="job-1,job-2"`) {
		t.Errorf("expected joined data-job-ids attribute, got: %s", got)
	}
}

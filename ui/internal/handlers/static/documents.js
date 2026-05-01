export function injectRetryForm(li, jobID, errorText) {
	const detail = li.querySelector(".detail");
	if (!detail || detail.querySelector("form.retry-form")) return;
	detail.textContent = errorText || "";
	const sep = document.createTextNode(" — ");
	const form = document.createElement("form");
	form.className = "retry-form";
	form.method = "POST";
	form.action = "/documents/" + jobID + "/retry";
	form.style.display = "inline";
	const btn = document.createElement("button");
	btn.type = "submit";
	btn.textContent = "Retry";
	form.appendChild(btn);
	detail.appendChild(sep);
	detail.appendChild(form);
}

export function injectDeleteForm(li, jobID) {
	const slot = li.querySelector(".delete-action");
	if (!slot || slot.querySelector("form.delete-form")) return;
	const filename = li.querySelector(".filename").textContent;
	const form = document.createElement("form");
	form.className = "delete-form";
	form.method = "POST";
	form.action = "/documents/" + jobID + "/delete";
	form.dataset.filename = filename;
	const btn = document.createElement("button");
	btn.type = "submit";
	btn.textContent = "Delete";
	form.appendChild(btn);
	form.addEventListener("submit", function (e) {
		if (!confirm("Delete " + filename + " ?")) e.preventDefault();
	});
	slot.appendChild(form);
}

export function watchJob(jobID) {
	const li = document.getElementById("job-" + jobID);
	if (!li) return;
	const es = new EventSource("/jobs/" + jobID + "/watch");
	es.addEventListener("status", function (e) {
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
			injectRetryForm(li, jobID, d.Error);
			injectDeleteForm(li, jobID);
			es.close();
		} else if (d.Stage) {
			let text = d.Stage;
			if (d.Message) text += " — " + d.Message;
			if (d.PagesTotal) text += " (" + d.PagesDone + "/" + d.PagesTotal + ")";
			detail.textContent = text;
		}
	});
}

function bootstrap() {
	document.querySelectorAll("form.delete-form").forEach(function (form) {
		form.addEventListener("submit", function (e) {
			if (!confirm("Delete " + form.dataset.filename + " ?")) e.preventDefault();
		});
	});

	const cfg = document.getElementById("watch-config");
	if (!cfg) return;
	cfg.dataset.jobIds.split(",").forEach(watchJob);
}

if (typeof document !== "undefined") {
	bootstrap();
}

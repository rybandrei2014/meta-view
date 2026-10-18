(() => {
  const $ = (s) => document.querySelector(s);
  const uploadView = $("#uploadView");
  const resultView = $("#resultView");
  const dropZone = $("#dropZone");
  const fileInput = $("#fileInput");
  const toast = $("#toast");
  let current = null;
  let toastTimer = null;

  const sectionIcons = {
    "File Properties": '<svg viewBox="0 0 24 24" width="17" height="17"><path d="M13 3H7a1 1 0 00-1 1v16a1 1 0 001 1h10a1 1 0 001-1V8l-5-5z" fill="none" stroke="currentColor" stroke-width="1.8"/><path d="M13 3v5h5" fill="none" stroke="currentColor" stroke-width="1.8"/></svg>',
    "Raw Data": '<svg viewBox="0 0 24 24" width="17" height="17"><path d="M8 6l-5 6 5 6M16 6l5 6-5 6" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>',
  };
  const defaultIcon = '<svg viewBox="0 0 24 24" width="17" height="17"><circle cx="12" cy="12" r="9" fill="none" stroke="currentColor" stroke-width="1.8"/><path d="M12 8v4m0 4h.01" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/></svg>';

  /* ---------- upload ---------- */
  $("#pickBtn").addEventListener("click", () => fileInput.click());
  fileInput.addEventListener("change", () => {
    if (fileInput.files.length) upload(fileInput.files[0]);
  });

  ["dragenter", "dragover"].forEach((ev) =>
    dropZone.addEventListener(ev, (e) => {
      e.preventDefault();
      dropZone.classList.add("drag");
    })
  );
  ["dragleave", "drop"].forEach((ev) =>
    dropZone.addEventListener(ev, (e) => {
      e.preventDefault();
      dropZone.classList.remove("drag");
    })
  );
  dropZone.addEventListener("drop", (e) => {
    const f = e.dataTransfer.files && e.dataTransfer.files[0];
    if (f) upload(f);
  });
  document.addEventListener("paste", (e) => {
    const f = e.clipboardData && e.clipboardData.files && e.clipboardData.files[0];
    if (f) upload(f);
  });

  async function upload(file) {
    dropZone.classList.add("loading");
    $("#pickBtn").disabled = true;
    try {
      const fd = new FormData();
      fd.append("file", file);
      const res = await fetch("/api/extract", { method: "POST", body: fd });
      if (!res.ok) throw new Error(await res.text());
      render(await res.json());
    } catch (err) {
      showToast(String(err.message || err));
    } finally {
      dropZone.classList.remove("loading");
      $("#pickBtn").disabled = false;
      fileInput.value = "";
    }
  }

  /* ---------- render ---------- */
  function render(r) {
    current = r;
    uploadView.hidden = true;
    resultView.hidden = false;
    $("#search").value = "";
    $("#clearSearch").hidden = true;
    $("#noMatches").hidden = true;

    $("#resName").textContent = r.fileName;
    $("#resSub").textContent = ` ${r.format} · ${r.sizeHuman} · ${r.count} properties`;

    const cards = [
      { label: "Type", value: r.format, sub: r.mime },
      { label: "Size", value: r.sizeHuman, sub: "on disk" },
      { label: "Dimensions", value: r.width ? `${r.width} × ${r.height}` : "—", sub: "width × height" },
      { label: "Properties", value: String(r.count), sub: `in ${r.sections.length} sections` },
    ];
    $("#cards").innerHTML = cards
      .map(
        (c) =>
          `<div class="card"><div class="label">${esc(c.label)}</div><div class="value">${esc(c.value)}</div><div class="sub">${esc(c.sub)}</div></div>`
      )
      .join("");

    const host = $("#sections");
    host.innerHTML = "";
    for (const sec of r.sections) {
      const det = document.createElement("details");
      det.className = "section";
      det.open = sec.name !== "Raw Data";
      const isRaw = sec.name === "Raw Data";
      det.innerHTML = `
        <summary>
          <svg class="chev" viewBox="0 0 24 24" width="14" height="14"><path d="M9 6l6 6-6 6" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>
          <span class="sec-icon">${sectionIcons[sec.name] || defaultIcon}</span>
          ${esc(sec.name)}
          <span class="count-badge">${sec.props.length}</span>
        </summary>
        <div class="props">
          ${sec.props
            .map(
              (p) =>
                `<div class="prop${isRaw ? " raw" : ""}"><span class="k">${esc(p.key)}</span><span class="v" title="Click to copy">${esc(p.value)}</span></div>`
            )
            .join("")}
        </div>`;
      host.appendChild(det);
    }
    applySearch();
  }

  /* ---------- interactions ---------- */
  $("#sections").addEventListener("click", (e) => {
    const v = e.target.closest(".v");
    if (!v) return;
    copy(v.textContent);
  });

  function copy(text) {
    const done = () => showToast("Copied to clipboard");
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(done, () => fallbackCopy(text, done));
    } else {
      fallbackCopy(text, done);
    }
  }
  function fallbackCopy(text, done) {
    const ta = document.createElement("textarea");
    ta.value = text;
    ta.style.position = "fixed";
    ta.style.opacity = "0";
    document.body.appendChild(ta);
    ta.select();
    try {
      document.execCommand("copy");
      done();
    } catch (_) {}
    ta.remove();
  }

  const search = $("#search");
  search.addEventListener("input", () => {
    $("#clearSearch").hidden = !search.value;
    applySearch();
  });
  $("#clearSearch").addEventListener("click", () => {
    search.value = "";
    $("#clearSearch").hidden = true;
    applySearch();
  });

  function applySearch() {
    const q = search.value.trim().toLowerCase();
    let anyVisible = false;
    for (const det of document.querySelectorAll("details.section")) {
      let visible = 0;
      for (const prop of det.querySelectorAll(".prop")) {
        const hit = !q || prop.textContent.toLowerCase().includes(q);
        prop.classList.toggle("hidden", !hit);
        if (hit) visible++;
      }
      det.classList.toggle("hidden", visible === 0);
      if (visible > 0) {
        anyVisible = true;
        if (q) det.open = true;
      }
    }
    $("#noMatches").hidden = anyVisible;
  }

  /* ---------- actions ---------- */
  $("#freshBtn").addEventListener("click", () => {
    current = null;
    resultView.hidden = true;
    uploadView.hidden = false;
  });

  $("#exportJson").addEventListener("click", () => {
    if (!current) return;
    download(current.fileName + ".metadata.json", JSON.stringify(current, null, 2), "application/json");
  });

  $("#exportCsv").addEventListener("click", () => {
    if (!current) return;
    const rows = ["section,key,value"];
    for (const s of current.sections)
      for (const p of s.props)
        rows.push([s.name, p.key, p.value].map(csvCell).join(","));
    download(current.fileName + ".metadata.csv", rows.join("\r\n"), "text/csv");
  });

  function csvCell(v) {
    v = String(v);
    return /[",\r\n]/.test(v) ? '"' + v.replace(/"/g, '""') + '"' : v;
  }

  function download(name, content, type) {
    const a = document.createElement("a");
    a.href = URL.createObjectURL(new Blob([content], { type }));
    a.download = name;
    a.click();
    setTimeout(() => URL.revokeObjectURL(a.href), 2000);
  }

  function showToast(msg) {
    toast.textContent = msg;
    toast.hidden = false;
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => (toast.hidden = true), 2200);
  }

  function esc(s) {
    return String(s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
  }

  /* preloaded file from CLI argument */
  fetch("/api/extract")
    .then((r) => (r.ok ? r.json() : null))
    .then((r) => r && render(r))
    .catch(() => {});
})();

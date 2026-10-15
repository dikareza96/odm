// How a tab's media list is ordered, described and drawn, shared by the
// in-page button (content.js) and the toolbar popup (popup.js) so both
// read alike. Loaded as a plain script in both places, and by Node for the
// tests (which use only the parts that need no DOM).
(function (root) {
  function fmtSize(n) {
    if (!n) return "";
    const u = ["B", "KB", "MB", "GB"];
    let i = 0;
    while (n >= 1000 && i < u.length - 1) {
      n /= 1024;
      i++;
    }
    return `${n.toFixed(i ? 2 : 0)} ${u[i]}`;
  }

  // Streams first: those known to list every quality (master), then in
  // the order the player loaded them, as a player reads the playlist
  // listing every quality before the one quality it plays. Files follow,
  // largest first.
  function orderMedia(items) {
    const streams = items.filter((i) => i.kind === "stream")
      .sort((a, b) => (b.master ? 1 : 0) - (a.master ? 1 : 0) || (a.at || 0) - (b.at || 0));
    const files = items.filter((i) => i.kind !== "stream").sort((a, b) => b.size - a.size);
    return [...streams, ...files];
  }

  // place is "host · file" of a URL, both shortened, for telling streams
  // apart when their playlist says little or could not be read.
  function place(url) {
    let u;
    try {
      u = new URL(url);
    } catch {
      return "";
    }
    let file = u.pathname.split("/").filter(Boolean).pop() || "";
    try {
      file = decodeURIComponent(file);
    } catch { /* a malformed escape: show it as it is */ }
    const cut = (s, n) => (s.length > n ? s.slice(0, n - 1) + "…" : s);
    const host = cut(u.hostname, 32);
    return file ? `${host} · ${cut(file, 28)}` : host;
  }

  // urlExt is the extension of a URL's last path segment ("webm"), or "".
  function urlExt(url) {
    try {
      const m = /\.([a-z0-9]{2,5})$/i.exec(new URL(url).pathname);
      return m ? m[1] : "";
    } catch {
      return "";
    }
  }

  // mediaParts splits an item into what a row shows: a title ("HLS
  // stream", "MP4 file"), a detail line, and flags for the LIVE and Best
  // badges. A stream's detail is what its playlist offers, with the host
  // and file added when that alone would not tell two streams apart.
  function mediaParts(it) {
    if (it.kind === "stream") {
      const where = place(it.url || "");
      let detail = it.quality || "";
      if (!/\d+p/.test(detail)) detail = [detail, where].filter(Boolean).join(" · ");
      return { title: `${it.format || "Media"} stream`, detail, live: !!it.live, best: !!it.master, stream: true };
    }
    // The MIME subtype names the format for video/ and audio/ types; for
    // anything else (application/octet-stream) the file extension does.
    const [major, sub] = (it.mime || "").split("/");
    let type = (major === "video" || major === "audio") && sub ? sub.replace(/^x-/, "") : "";
    if (!type) type = urlExt(it.url) || (major === "audio" ? "audio" : "video");
    type = type.toUpperCase();
    return { title: `${type} file`, detail: fmtSize(it.size), live: false, best: false, stream: false };
  }

  // mediaLabel is mediaParts on one line, for places without rows.
  function mediaLabel(it) {
    const p = mediaParts(it);
    return [p.title, p.detail, p.live ? "live" : ""].filter(Boolean).join(" · ");
  }

  // "12s ago", "3 min ago"; "" without a time.
  function ago(at, now) {
    if (!at) return "";
    const s = Math.max(0, Math.round((now - at) / 1000));
    if (s < 60) return `${s}s ago`;
    if (s < 3600) return `${Math.round(s / 60)} min ago`;
    return `${Math.round(s / 3600)} h ago`;
  }

  /* ---------- drawing ---------- */

  // Icons are fixed markup, never built from page data.
  const ICONS = {
    download: '<svg viewBox="0 0 16 16" aria-hidden="true"><path fill="currentColor" d="M7.25 1.75a.75.75 0 0 1 1.5 0v6.69l2.22-2.22a.75.75 0 1 1 1.06 1.06l-3.5 3.5a.75.75 0 0 1-1.06 0l-3.5-3.5a.75.75 0 0 1 1.06-1.06l2.22 2.22V1.75ZM2.75 12a.75.75 0 0 1 .75.75v.75h9v-.75a.75.75 0 0 1 1.5 0v1.5a.75.75 0 0 1-.75.75H2.75a.75.75 0 0 1-.75-.75v-1.5a.75.75 0 0 1 .75-.75Z"/></svg>',
    close: '<svg viewBox="0 0 16 16" aria-hidden="true"><path fill="currentColor" d="M3.72 3.72a.75.75 0 0 1 1.06 0L8 6.94l3.22-3.22a.75.75 0 1 1 1.06 1.06L9.06 8l3.22 3.22a.75.75 0 1 1-1.06 1.06L8 9.06l-3.22 3.22a.75.75 0 0 1-1.06-1.06L6.94 8 3.72 4.78a.75.75 0 0 1 0-1.06Z"/></svg>',
    stream: '<svg viewBox="0 0 16 16" aria-hidden="true"><path fill="currentColor" d="M2 3.75C2 2.78 2.78 2 3.75 2h8.5c.97 0 1.75.78 1.75 1.75v8.5c0 .97-.78 1.75-1.75 1.75h-8.5C2.78 14 2 13.22 2 12.25v-8.5Zm4.5 1.9v4.7c0 .4.43.64.77.43l3.76-2.35a.5.5 0 0 0 0-.86L7.27 5.22a.5.5 0 0 0-.77.43Z"/></svg>',
    file: '<svg viewBox="0 0 16 16" aria-hidden="true"><path fill="currentColor" d="M2.5 1.75C2.5.78 3.28 0 4.25 0h4.69c.46 0 .9.18 1.23.51l2.82 2.82c.33.33.51.77.51 1.23v9.69c0 .97-.78 1.75-1.75 1.75h-7.5c-.97 0-1.75-.78-1.75-1.75V1.75Zm4 6.4v3.7c0 .38.41.62.74.43l3.2-1.85a.5.5 0 0 0 0-.86l-3.2-1.85a.5.5 0 0 0-.74.43Z"/></svg>',
  };

  // ROW_CSS styles mediaRow inside a page that defines these tokens:
  // --odm-fg, --odm-muted, --odm-hover, --odm-icon-bg, --odm-accent.
  const ROW_CSS = `
    .odm-row { display: flex; align-items: center; gap: 10px; width: 100%; box-sizing: border-box;
      padding: 8px 10px; border-radius: 8px; cursor: pointer; text-align: left;
      background: none; border: 0; color: var(--odm-fg); font: inherit; }
    .odm-row:hover, .odm-row:focus-visible { background: var(--odm-hover); outline: none; }
    .odm-ic { flex: none; width: 30px; height: 30px; border-radius: 7px; display: grid; place-items: center;
      background: var(--odm-icon-bg); color: var(--odm-muted); }
    .odm-ic.odm-stream { color: var(--odm-accent); }
    .odm-ic svg { width: 16px; height: 16px; }
    .odm-txt { flex: 1; min-width: 0; display: flex; flex-direction: column; }
    .odm-t { display: flex; align-items: center; gap: 6px; font-weight: 600; font-size: 13px; line-height: 18px; }
    .odm-d { color: var(--odm-muted); font-size: 12px; line-height: 16px;
      white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
    .odm-tag { font-size: 10px; font-weight: 700; letter-spacing: .04em; line-height: 14px;
      padding: 0 5px; border-radius: 4px; flex: none; }
    .odm-live { background: #da3633; color: #fff; }
    .odm-best { color: #3fb950; box-shadow: inset 0 0 0 1px rgba(63, 185, 80, .45); }
    .odm-when { flex: none; color: var(--odm-muted); font-size: 11px; }
  `;

  // mediaRow builds one clickable row. Page data only ever goes through
  // textContent and title.
  function mediaRow(doc, it, opts) {
    const p = mediaParts(it);
    const row = doc.createElement("button");
    row.type = "button";
    row.className = "odm-row";
    row.title = it.url || "";
    const ic = doc.createElement("span");
    ic.className = "odm-ic" + (p.stream ? " odm-stream" : "");
    ic.innerHTML = p.stream ? ICONS.stream : ICONS.file;
    const txt = doc.createElement("span");
    txt.className = "odm-txt";
    const t = doc.createElement("span");
    t.className = "odm-t";
    const name = doc.createElement("span");
    name.textContent = p.title;
    t.append(name);
    if (p.live) t.append(tag(doc, "LIVE", "odm-live"));
    if (p.best) t.append(tag(doc, "Best", "odm-best"));
    txt.append(t);
    if (p.detail) {
      const d = doc.createElement("span");
      d.className = "odm-d";
      d.textContent = p.detail;
      txt.append(d);
    }
    row.append(ic, txt);
    if (opts && opts.now && it.at) {
      const w = doc.createElement("span");
      w.className = "odm-when";
      w.textContent = ago(it.at, opts.now);
      row.append(w);
    }
    return row;
  }

  function tag(doc, text, cls) {
    const s = doc.createElement("span");
    s.className = "odm-tag " + cls;
    s.textContent = text;
    return s;
  }

  const api = { fmtSize, orderMedia, mediaParts, mediaLabel, place, ago, mediaRow, ROW_CSS, ICONS };
  // Always set: in a content script, a page element with id="module"
  // makes the global "module" look defined.
  root.odmMediaList = api;
  if (typeof module !== "undefined" && module.exports) module.exports = api;
})(typeof self !== "undefined" ? self : this);

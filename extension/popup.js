const enabled = document.getElementById("enabled");
const status = document.getElementById("status");
const dot = document.getElementById("dot");
const { orderMedia, mediaRow, ROW_CSS, ICONS } = self.odmMediaList;

// The rows' styles come with them from medialist.js.
const rowStyle = document.createElement("style");
rowStyle.textContent = ROW_CSS;
document.head.appendChild(rowStyle);
document.getElementById("logo").innerHTML = ICONS.download;

// say shows a message; reach also colours the connection dot, which is
// only about whether ODM answered, not whether it accepted a download.
function say(text, kind, reach) {
  status.textContent = text;
  status.className = kind || "";
  if (reach) dot.className = "dot " + reach;
}

chrome.storage.local.get("enabled").then(({ enabled: v = true }) => (enabled.checked = v));

enabled.onchange = () => chrome.storage.local.set({ enabled: enabled.checked });

function testConnection() {
  try {
    const port = chrome.runtime.connectNative("com.ochi.odm");
    port.onMessage.addListener((msg) => {
      if (msg && msg.ok) say("ODM is reachable", "ok", "ok");
      else say(`reply: ${JSON.stringify(msg)}`, "err", "ok");
      port.disconnect();
    });
    port.onDisconnect.addListener(() => {
      if (chrome.runtime.lastError) say("Cannot reach ODM (is it running?)", "err", "err");
    });
    port.postMessage({ type: "ping" });
  } catch (e) {
    say(String(e), "err", "err");
  }
}
document.getElementById("test").onclick = testConnection;

// Media the tab has loaded, from every frame: also what the in-page button
// cannot offer (audio-only pages, canvas players, a frame without a large
// <video>, or the button closed with ✕).
const mediaBox = document.getElementById("media");

chrome.tabs.query({ active: true, currentWindow: true }).then(([tab]) => {
  if (!tab) return;
  chrome.runtime.sendMessage({ type: "odm-tab-media", tabId: tab.id }, (list) => {
    if (chrome.runtime.lastError || !Array.isArray(list) || list.length === 0) return;
    document.getElementById("count").textContent = String(list.length);
    const now = Date.now();
    mediaBox.replaceChildren();
    for (const it of orderMedia(list)) {
      const row = mediaRow(document, it, { now });
      row.onclick = () => {
        say("Sending…");
        chrome.runtime.sendMessage({ type: "odm-grab", item: it, tabId: tab.id }, (reply) => {
          if (chrome.runtime.lastError || !reply) say("ODM is not reachable", "err", "err");
          else if (!reply.ok) say(`ODM: ${reply.error}`, "err", "ok");
          else say(reply.message === "already in the list" ? "Already in ODM" : "Sent to ODM", "ok", "ok");
        });
      };
      mediaBox.appendChild(row);
    }
  });
});

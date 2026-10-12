const enabled = document.getElementById("enabled");
const status = document.getElementById("status");

chrome.storage.local.get("enabled").then(({ enabled: v = true }) => (enabled.checked = v));

enabled.onchange = () => chrome.storage.local.set({ enabled: enabled.checked });

document.getElementById("test").onclick = () => {
  try {
    const port = chrome.runtime.connectNative("com.ochi.odm");
    port.onMessage.addListener((msg) => {
      status.textContent = msg && msg.ok ? "ODM is reachable" : `reply: ${JSON.stringify(msg)}`;
      port.disconnect();
    });
    port.onDisconnect.addListener(() => {
      if (chrome.runtime.lastError) status.textContent = "Cannot reach ODM (is it running?)";
    });
    port.postMessage({ type: "ping" });
  } catch (e) {
    status.textContent = String(e);
  }
};

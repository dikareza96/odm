// ODM Integration — routes browser downloads to Ochi Download Manager through
// the native-messaging host (ODM.exe -native-messaging).

const HOST = "com.ochi.odm";
const FILE_RE = /\.(zip|rar|7z|tar|gz|xz|bz2|zst|exe|msi|dmg|pkg|deb|rpm|apk|iso|img|vhd|mp4|mkv|avi|mov|webm|flv|m4v|wmv|mp3|m4a|aac|flac|wav|ogg|pdf|epub|doc|docx|xls|xlsx|ppt|pptx|csv|txt|bin|torrent)$/i;

let port = null;

function connect() {
  if (port) return port;
  port = chrome.runtime.connectNative(HOST);
  port.onDisconnect.addListener(() => { port = null; });
  return port;
}

function sendToOdm(payload) {
  try {
    connect().postMessage(payload);
    return true;
  } catch (e) {
    return false;
  }
}

async function isEnabled() {
  const { enabled = true } = await chrome.storage.local.get("enabled");
  return enabled;
}

function looksDownloadable(url) {
  try {
    return FILE_RE.test(new URL(url).pathname);
  } catch {
    return false;
  }
}

chrome.downloads.onCreated.addListener(async (item) => {
  if (!(await isEnabled())) return;
  if (!item.url || !item.url.startsWith("http")) return;
  if (!looksDownloadable(item.url)) return;
  if (sendToOdm({ type: "download", url: item.url, referrer: item.referrer || "" })) {
    chrome.downloads.cancel(item.id, () => chrome.downloads.erase({ id: item.id }));
  }
});

chrome.runtime.onInstalled.addListener(() => {
  chrome.contextMenus.create({
    id: "odm-download",
    title: "Download with ODM",
    contexts: ["link", "video", "audio", "image"],
  });
});

chrome.contextMenus.onClicked.addListener((info) => {
  const url = info.linkUrl || info.srcUrl;
  if (url) sendToOdm({ type: "download", url, referrer: info.pageUrl || "" });
});

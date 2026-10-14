// Run with: node --test extension/test/*.test.js
const test = require("node:test");
const assert = require("node:assert/strict");
const bg = require("../background.js");

test("mediaKind", () => {
  assert.equal(bg.mediaKind("https://rr2---sn.googlevideo.com/videoplayback?expire=1", "video/mp4"), "file");
  assert.equal(bg.mediaKind("https://cdn/x/master.m3u8", ""), "stream");
  assert.equal(bg.mediaKind("https://cdn/x/manifest", "application/dash+xml"), "stream");
  assert.equal(bg.mediaKind("https://cdn/x/seg-001.ts", "video/mp2t"), null); // HLS segment
  assert.equal(bg.mediaKind("https://cdn/x/chunk-5.m4s", "video/mp4"), null); // fMP4 segment
  assert.equal(bg.mediaKind("https://cdn/x/seg", "audio/iso.segment"), null);
  assert.equal(bg.mediaKind("https://cdn/clip.mp4", "application/octet-stream"), "file");
  assert.equal(bg.mediaKind("https://cdn/page.html", "text/html"), null);
  assert.equal(bg.mediaKind("https://cdn/song", "audio/mpeg; charset=x"), "file");
});

test("normalizeMediaURL drops Google's byte-range parameters only", () => {
  const g = bg.normalizeMediaURL("https://rr2---sn-a.googlevideo.com/videoplayback?expire=9&itag=18&range=0-1048575&rn=3&rbuf=0#t=1");
  const u = new URL(g);
  assert.equal(u.searchParams.get("range"), null);
  assert.equal(u.searchParams.get("rn"), null);
  assert.equal(u.searchParams.get("itag"), "18");
  assert.equal(u.hash, "");
  assert.equal(bg.normalizeMediaURL("https://other.example/v.mp4?range=5"), "https://other.example/v.mp4?range=5");
});

test("totalSize", () => {
  assert.equal(bg.totalSize([{ name: "Content-Range", value: "bytes 0-1048575/69652480" }]), 69652480);
  assert.equal(bg.totalSize([{ name: "content-length", value: "1234" }]), 1234);
  assert.equal(bg.totalSize([]), 0);
  assert.equal(bg.totalSize([{ name: "Content-Range", value: "bytes 0-9/*" }, { name: "Content-Length", value: "10" }]), 10);
});

test("exclusions", () => {
  assert.ok(bg.hostMatches("download.windowsupdate.com", "*.windowsupdate.com"));
  assert.ok(bg.hostMatches("windowsupdate.com", "*.windowsupdate.com"));
  assert.ok(!bg.hostMatches("evilwindowsupdate.com", "*.windowsupdate.com"));
  assert.ok(bg.isExcluded("https://Example.org/a.zip", ["example.org"]));
  assert.ok(!bg.isExcluded("https://sub.example.org/a.zip", ["example.org"]));
});

test("hasType checks the URL and the browser's file name", () => {
  assert.ok(bg.hasType("https://h/a.ZIP?x=1", ["zip"]));
  assert.ok(bg.hasType("https://h/download?id=3", [".exe"], "C:\\Users\\u\\Downloads\\setup.exe"));
  assert.ok(!bg.hasType("https://h/page", ["zip"], ""));
});

test("mergeMedia dedupes, keeps the larger size, caps the list", () => {
  let list = bg.mergeMedia([], { url: "a", size: 100 });
  list = bg.mergeMedia(list, { url: "a", size: 0 });
  assert.equal(list.length, 1);
  assert.equal(list[0].size, 100);
  for (let i = 0; i < 40; i++) list = bg.mergeMedia(list, { url: "u" + i, size: i });
  assert.equal(list.length, 15);
  assert.equal(list[0].url, "u39");
});

test("mergeMedia keeps when an item was first seen", () => {
  let list = bg.mergeMedia([], { url: "m", size: 0, at: 5 });
  list = bg.mergeMedia(list, { url: "m", size: 0, at: 9 });
  assert.equal(list[0].at, 5);
  assert.equal(bg.mergeMedia([], { url: "x", size: 0 })[0].at, undefined);
});

test("mergeMedia drops files before playlists when full", () => {
  let list = bg.mergeMedia([], { url: "master", kind: "stream", size: 0 });
  for (let i = 0; i < 40; i++) list = bg.mergeMedia(list, { url: "f" + i, kind: "file", size: i });
  assert.equal(list.length, 15);
  assert.ok(list.some((x) => x.url === "master"));
  assert.equal(list[0].url, "f39");
});

test("streamFormat", () => {
  assert.equal(bg.streamFormat("https://cdn/x/master.m3u8", ""), "HLS");
  assert.equal(bg.streamFormat("https://cdn/x/play", "application/x-mpegURL; charset=utf-8"), "HLS");
  assert.equal(bg.streamFormat("https://cdn/x/manifest", "application/dash+xml"), "DASH");
  assert.equal(bg.streamFormat("https://cdn/x/m.mpd?t=1", "text/plain"), "DASH");
  assert.equal(bg.streamFormat("https://cdn/x/video", "video/mp4"), "");
});

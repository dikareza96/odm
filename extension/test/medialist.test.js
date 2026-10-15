const test = require("node:test");
const assert = require("node:assert/strict");
const ml = require("../medialist.js");

test("orderMedia puts streams first, oldest first, then files by size", () => {
  const got = ml.orderMedia([
    { url: "f1", kind: "file", size: 10 },
    { url: "s2", kind: "stream", at: 20 },
    { url: "f2", kind: "file", size: 99 },
    { url: "s1", kind: "stream", at: 10 },
  ]).map((x) => x.url);
  assert.deepEqual(got, ["s1", "s2", "f2", "f1"]);
});

test("mediaLabel", () => {
  assert.equal(ml.mediaLabel({ kind: "stream", format: "HLS" }), "HLS stream");
  assert.equal(ml.mediaLabel({ kind: "stream" }), "Media stream");
  assert.equal(ml.mediaLabel({ kind: "file", mime: "video/mp4", size: 1536 }), "MP4 file · 1.50 KB");
  assert.equal(ml.mediaLabel({ kind: "file", mime: "video/x-matroska", size: 0 }), "MATROSKA file");
  assert.equal(ml.mediaLabel({ kind: "file", size: 0 }), "VIDEO file");
  assert.equal(ml.mediaLabel({ kind: "file", mime: "application/octet-stream", url: "https://c.example/v/clip.webm?x=1", size: 0 }), "WEBM file");
});

test("ago", () => {
  assert.equal(ml.ago(0, 5000), "");
  assert.equal(ml.ago(1000, 13000), "12s ago");
  assert.equal(ml.ago(0 + 1, 1 + 180000), "3 min ago");
  assert.equal(ml.ago(1, 1 + 7200000), "2 h ago");
});

test("stream parts and order", () => {
  const twitch = "https://usher.ttvnw.net/api/channel/hls/blastdota.m3u8?sig=1";
  assert.deepEqual(ml.mediaParts({ kind: "stream", format: "HLS", url: twitch, quality: "All qualities, up to 1080p", live: true, master: true }),
    { title: "HLS stream", detail: "All qualities, up to 1080p", live: true, best: true, stream: true });
  // Without a height the host and file tell streams apart.
  assert.equal(ml.mediaParts({ kind: "stream", format: "HLS", url: twitch, quality: "One quality" }).detail,
    "One quality · usher.ttvnw.net · blastdota.m3u8");
  assert.equal(ml.mediaParts({ kind: "stream", format: "HLS", url: twitch }).detail, "usher.ttvnw.net · blastdota.m3u8");
  assert.equal(ml.place("https://h.example/a/very-long-playlist-name-that-goes-on.m3u8"), "h.example · very-long-playlist-name-tha…");
  assert.equal(ml.place("https://h.example/a%E0b.m3u8"), "h.example · a%E0b.m3u8");
  assert.equal(ml.mediaLabel({ kind: "stream", format: "HLS", quality: "One quality, 720p", live: true }), "HLS stream · One quality, 720p · live");
  const got = ml.orderMedia([
    { url: "one", kind: "stream", at: 1 },
    { url: "all", kind: "stream", at: 9, master: true },
  ]).map((x) => x.url);
  assert.deepEqual(got, ["all", "one"]);
});

// Cubbit Pages — Service Worker for transparent .enc decryption
'use strict';

var MAGIC = [0x43, 0x50, 0x47, 0x53]; // CPGS
var VERSION = 1;
var SALT_LEN = 16;
var NONCE_LEN = 12;
var HEADER_LEN = 4 + 1 + SALT_LEN + NONCE_LEN; // 33
var ITERATIONS = 100000;

var password = null;
var CACHE_NAME = 'cubbit-pages-v1';

// MIME type map (matches upload.go)
var MIME_TYPES = {
  '.html': 'text/html; charset=utf-8',
  '.htm': 'text/html; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.js': 'application/javascript',
  '.mjs': 'application/javascript',
  '.json': 'application/json',
  '.svg': 'image/svg+xml',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.jpeg': 'image/jpeg',
  '.gif': 'image/gif',
  '.webp': 'image/webp',
  '.avif': 'image/avif',
  '.ico': 'image/x-icon',
  '.woff': 'font/woff',
  '.woff2': 'font/woff2',
  '.ttf': 'font/ttf',
  '.otf': 'font/otf',
  '.eot': 'application/vnd.ms-fontobject',
  '.txt': 'text/plain; charset=utf-8',
  '.xml': 'application/xml',
  '.pdf': 'application/pdf',
  '.mp4': 'video/mp4',
  '.webm': 'video/webm',
  '.mp3': 'audio/mpeg',
  '.ogg': 'audio/ogg',
  '.wasm': 'application/wasm',
  '.map': 'application/json'
};

function getContentType(url) {
  var path = new URL(url).pathname;
  // Remove .enc suffix to get original extension
  if (path.endsWith('.enc')) {
    path = path.slice(0, -4);
  }
  var dot = path.lastIndexOf('.');
  if (dot === -1) return 'application/octet-stream';
  var ext = path.slice(dot).toLowerCase();
  return MIME_TYPES[ext] || 'application/octet-stream';
}

function decryptData(data, pwd) {
  if (data.length < HEADER_LEN) return Promise.reject('too short');
  for (var i = 0; i < 4; i++) {
    if (data[i] !== MAGIC[i]) return Promise.reject('bad magic');
  }
  if (data[4] !== VERSION) return Promise.reject('bad version');
  var off = 5;
  var salt = data.slice(off, off + SALT_LEN); off += SALT_LEN;
  var nonce = data.slice(off, off + NONCE_LEN); off += NONCE_LEN;
  var ct = data.slice(off);
  return deriveKey(pwd, salt).then(function(key) {
    return crypto.subtle.decrypt({ name: 'AES-GCM', iv: nonce, tagLength: 128 }, key, ct);
  });
}

function deriveKey(pwd, salt) {
  var enc = new TextEncoder();
  return crypto.subtle.importKey('raw', enc.encode(pwd), 'PBKDF2', false, ['deriveKey']).then(function(km) {
    return crypto.subtle.deriveKey(
      { name: 'PBKDF2', salt: salt, iterations: ITERATIONS, hash: 'SHA-256' },
      km,
      { name: 'AES-GCM', length: 256 },
      false,
      ['decrypt']
    );
  });
}

// Activate immediately, claim all clients
self.addEventListener('install', function(e) {
  e.waitUntil(self.skipWaiting());
});

self.addEventListener('activate', function(e) {
  e.waitUntil(self.clients.claim());
});

// Receive password from login page
self.addEventListener('message', function(e) {
  if (e.data && e.data.type === 'SET_PASSWORD') {
    password = e.data.password;
    // Clear cached decrypted files when password changes
    caches.delete(CACHE_NAME);
    // Confirm via MessageChannel port if available, else via source
    var reply = { type: 'PASSWORD_SET' };
    if (e.ports && e.ports[0]) {
      e.ports[0].postMessage(reply);
    } else if (e.source) {
      e.source.postMessage(reply);
    }
  }
});

self.addEventListener('fetch', function(e) {
  // Only handle same-origin GET requests
  if (e.request.method !== 'GET') return;
  var url = new URL(e.request.url);
  if (url.origin !== self.location.origin) return;

  // Never intercept the SW itself, login page, or _verify.enc
  var path = url.pathname;
  var scope = self.registration.scope;
  var scopePath = new URL(scope).pathname;

  // Get relative path within scope
  var relPath = path;
  if (path.startsWith(scopePath)) {
    relPath = path.slice(scopePath.length);
  }

  // Don't intercept: sw.js, login page (index.html at root), _verify.enc
  if (relPath === 'sw.js' || relPath === '' || relPath === 'index.html' || relPath === '_verify.enc') {
    return;
  }

  // Don't intercept .enc files — those are fetched directly by this SW
  if (path.endsWith('.enc')) return;

  if (!password) return;

  e.respondWith(
    caches.open(CACHE_NAME).then(function(cache) {
      return cache.match(e.request).then(function(cached) {
        if (cached) return cached;

        // Try the original URL first (in case it exists unencrypted)
        return fetch(e.request).then(function(response) {
          if (response.ok) return response;
          // Not found — try .enc version
          return fetchAndDecrypt(e.request.url, cache);
        }).catch(function() {
          // Network error — try .enc version
          return fetchAndDecrypt(e.request.url, cache);
        });
      });
    })
  );
});

function fetchAndDecrypt(originalUrl, cache) {
  var encUrl = originalUrl + '.enc';
  return fetch(encUrl).then(function(r) {
    if (!r.ok) {
      return new Response('Not Found', { status: 404, statusText: 'Not Found' });
    }
    return r.arrayBuffer();
  }).then(function(buf) {
    if (buf instanceof Response) return buf;
    return decryptData(new Uint8Array(buf), password);
  }).then(function(plain) {
    if (plain instanceof Response) return plain;
    var contentType = getContentType(originalUrl);
    var response = new Response(plain, {
      status: 200,
      headers: { 'Content-Type': contentType }
    });
    // Cache the decrypted response
    cache.put(new Request(originalUrl), response.clone());
    return response;
  }).catch(function() {
    return new Response('Decryption Failed', { status: 500, statusText: 'Decryption Failed' });
  });
}

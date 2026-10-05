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
var DB_NAME = 'cubbit-pages';
var DB_STORE = 'auth';
var DB_KEY = 'password';

// --- Download/decrypt progress ---
// _manifest.json (generated at deploy time) lists the encrypted assets and
// their sizes plus a total. As the SW streams/decrypts each asset we add its
// bytes to progress.received, capped at the size declared in the manifest, so
// the overlay can show "Loading 13.2 MB / 50.1 MB". Capping also prevents a
// resource fetched twice (e.g. an <img> plus a JS fetch) from double-counting.
var manifest = null;
var manifestTried = false; // ensures a missing manifest is not refetched per request
var counted = {}; // manifest key ("path.ext.enc") -> bytes counted so far
var progress = { received: 0, total: 0 };

function resetProgress() {
  counted = {};
  progress = { received: 0, total: manifest && manifest.total ? manifest.total : 0 };
}

function loadManifest() {
  if (manifestTried) return Promise.resolve(manifest);
  manifestTried = true;
  return fetch('_manifest.json').then(function(r) {
    if (!r.ok) throw new Error('no manifest');
    return r.json();
  }).then(function(m) {
    manifest = m;
    progress.total = m && m.total ? m.total : 0;
    return m;
  }).catch(function() {
    manifest = null;
    return null;
  });
}

function relKeyFromUrl(u) {
  var path = new URL(u).pathname;
  var scopePath = new URL(self.registration.scope).pathname;
  if (path.indexOf(scopePath) === 0) path = path.slice(scopePath.length);
  return path;
}

// Manifest keys always carry the .enc extension; accept either form.
function encKey(relKey) {
  return relKey.slice(-4) === '.enc' ? relKey : relKey + '.enc';
}

function declaredSize(key) {
  if (!manifest || !manifest.files) return 0;
  var s = manifest.files[key];
  return typeof s === 'number' ? s : 0;
}

// addBytes credits n bytes to a resource, never exceeding its declared size.
function addBytes(relKey, n) {
  if (!manifest || !manifest.files || n <= 0) return;
  var key = encKey(relKey);
  var declared = declaredSize(key);
  if (declared === 0) return;
  var have = counted[key] || 0;
  var room = declared - have;
  if (room <= 0) return;
  if (n > room) n = room;
  counted[key] = have + n;
  progress.received += n;
}

// Read a response body chunk by chunk, crediting each chunk to progress as it
// arrives, so the overlay shows bytes accumulating instead of jumping at the
// end. Returns the full body as a Uint8Array.
function readWithProgress(response, relKey) {
  if (!response.body || !response.body.getReader) {
    return response.arrayBuffer().then(function(b) { return new Uint8Array(b); });
  }
  var reader = response.body.getReader();
  var chunks = [];
  var total = 0;
  function pump() {
    return reader.read().then(function(res) {
      if (res.done) {
        var out = new Uint8Array(total);
        var off = 0;
        for (var i = 0; i < chunks.length; i++) { out.set(chunks[i], off); off += chunks[i].length; }
        return out;
      }
      var chunk = res.value;
      chunks.push(chunk);
      total += chunk.length;
      addBytes(relKey, chunk.length);
      return pump();
    });
  }
  return pump();
}

// --- IndexedDB persistence for password ---
// The SW can be terminated by the browser at any time. When restarted,
// the in-memory password variable is null. We persist to IndexedDB
// (which the SW can access, unlike localStorage) so the password
// survives SW restarts without requiring a round-trip to clients.

function openDB() {
  return new Promise(function(resolve, reject) {
    var req = indexedDB.open(DB_NAME, 1);
    req.onupgradeneeded = function(e) {
      e.target.result.createObjectStore(DB_STORE);
    };
    req.onsuccess = function(e) { resolve(e.target.result); };
    req.onerror = function(e) { reject(e.target.error); };
  });
}

function savePassword(pwd) {
  return openDB().then(function(db) {
    return new Promise(function(resolve, reject) {
      var tx = db.transaction(DB_STORE, 'readwrite');
      tx.objectStore(DB_STORE).put(pwd, DB_KEY);
      tx.oncomplete = function() { resolve(); };
      tx.onerror = function(e) { reject(e.target.error); };
    });
  });
}

function loadPassword() {
  return openDB().then(function(db) {
    return new Promise(function(resolve, reject) {
      var tx = db.transaction(DB_STORE, 'readonly');
      var req = tx.objectStore(DB_STORE).get(DB_KEY);
      req.onsuccess = function() { resolve(req.result || null); };
      req.onerror = function(e) { reject(e.target.error); };
    });
  });
}

function clearPassword() {
  return openDB().then(function(db) {
    return new Promise(function(resolve, reject) {
      var tx = db.transaction(DB_STORE, 'readwrite');
      tx.objectStore(DB_STORE).delete(DB_KEY);
      tx.oncomplete = function() { resolve(); };
      tx.onerror = function(e) { reject(e.target.error); };
    });
  });
}

// --- MIME type map (matches upload.go) ---
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
    // Persist to IndexedDB so it survives SW restarts
    savePassword(password);
    // Clear cached decrypted files when password changes
    caches.delete(CACHE_NAME);
    resetProgress();
    manifestTried = false;
    loadManifest();
    // Confirm via MessageChannel port if available, else via source
    var reply = { type: 'PASSWORD_SET' };
    if (e.ports && e.ports[0]) {
      e.ports[0].postMessage(reply);
    } else if (e.source) {
      e.source.postMessage(reply);
    }
  }
  if (e.data && e.data.type === 'CLEAR_PASSWORD') {
    password = null;
    clearPassword();
    caches.delete(CACHE_NAME);
    resetProgress();
  }
  if (e.data && e.data.type === 'ADD_BYTES') {
    // The initial page is fetched directly by the login/loader page (not
    // through this SW), so it reports its own key and byte count here.
    // Make sure the manifest is loaded before crediting, otherwise the bytes
    // would be dropped (the message can arrive before loadManifest resolves).
    var bytes = e.data.bytes || 0;
    var key = e.data.key;
    var apply = function() { if (key) addBytes(key, bytes); };
    if (manifest) { apply(); } else { loadManifest().then(apply); }
  }
  if (e.data && e.data.type === 'GET_PROGRESS') {
    var pr = { type: 'PROGRESS', received: progress.received, total: progress.total };
    if (e.ports && e.ports[0]) e.ports[0].postMessage(pr);
  }
});

// Ensure password is available, restoring from IndexedDB if needed, and the
// manifest is loaded (progress accounting depends on it, so requests must not
// be served before it is ready).
function ensurePassword() {
  var pwdPromise;
  if (password) {
    pwdPromise = Promise.resolve(password);
  } else {
    pwdPromise = loadPassword().then(function(pwd) {
      if (pwd) password = pwd;
      return pwd;
    }).catch(function() {
      return null;
    });
  }
  var manifestPromise = manifest ? Promise.resolve(manifest) : loadManifest();
  return Promise.all([pwdPromise, manifestPromise]).then(function(r) {
    return r[0];
  });
}

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

  // Don't intercept: sw.js, login page (index.html at root), _verify.enc, _manifest.json
  if (relPath === 'sw.js' || relPath === '' || relPath === 'index.html' || relPath === '_verify.enc' || relPath === '_manifest.json') {
    return;
  }

  // Don't intercept .enc files — those are fetched directly by this SW
  if (path.endsWith('.enc')) return;

  e.respondWith(
    ensurePassword().then(function(pwd) {
      if (!pwd) {
        // No password available — let the request through unmodified
        return fetch(e.request);
      }

      return caches.open(CACHE_NAME).then(function(cache) {
        return cache.match(e.request).then(function(cached) {
          if (cached) {
            // A cache hit transfers no bytes, but the resource is loaded, so
            // credit it in full: the overlay label is "Loading", and this lets
            // a fully-cached revisit reach 100% and dismiss. addBytes caps at
            // the declared size, so this can never exceed the total.
            var hitKey = encKey(relKeyFromUrl(e.request.url));
            addBytes(hitKey, declaredSize(hitKey));
            return cached;
          }

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
    return readWithProgress(r, relKeyFromUrl(encUrl));
  }).then(function(buf) {
    if (buf instanceof Response) return buf;
    return decryptData(buf, password);
  }).then(function(plain) {
    if (plain instanceof Response) return plain;
    var contentType = getContentType(originalUrl);
    var response = new Response(plain, {
      status: 200,
      headers: { 'Content-Type': contentType }
    });
    // Cache the decrypted response
    cache.put(new Request(originalUrl), response.clone());
    // If the stream was shorter than declared (e.g. cached/range), top up.
    var key = encKey(relKeyFromUrl(originalUrl));
    addBytes(key, declaredSize(key) - (counted[key] || 0));
    return response;
  }).catch(function() {
    return new Response('Decryption Failed', { status: 500, statusText: 'Decryption Failed' });
  });
}

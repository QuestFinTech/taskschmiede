// Taskschmiede Documentation Search (client-side)
// Loads search-index.json and searches locally in the browser.
// Can be replaced with server-side PHP search for query logging on production.
//
// Copyright 2026 Quest Financial Technologies S.a r.l.-S., Luxembourg
// Licensed under the Apache License, Version 2.0

(function () {
  'use strict';

  var wrappers = document.querySelectorAll('.ts-search-wrapper');
  if (!wrappers.length) return;

  var index = null;
  var maxResults = 20;
  var minQueryLength = 2;

  function loadIndex() {
    if (index !== null) return;
    fetch('/search-index.json')
      .then(function (r) { return r.json(); })
      .then(function (data) { index = data; })
      .catch(function () { index = []; });
  }

  wrappers.forEach(function (wrapper) {
    var input = wrapper.querySelector('input');
    var results = wrapper.querySelector('.ts-search-results');
    if (!input || !results) return;

    var debounceTimer = null;

    input.addEventListener('focus', loadIndex);

    input.addEventListener('input', function () {
      clearTimeout(debounceTimer);
      var query = input.value.trim();
      if (query.length < minQueryLength) {
        results.innerHTML = '';
        results.style.display = 'none';
        return;
      }
      debounceTimer = setTimeout(function () {
        doSearch(query, results);
      }, 150);
    });

    input.addEventListener('keydown', function (e) {
      if (e.key === 'Escape') {
        results.innerHTML = '';
        results.style.display = 'none';
        input.blur();
      }
      if (e.key === 'Enter') {
        e.preventDefault();
        var firstLink = results.querySelector('a.ts-search-item');
        if (firstLink) {
          window.location.href = firstLink.getAttribute('href');
        }
      }
    });

    // Prevent parent form (Docsy sidebar) from submitting on Enter
    var parentForm = wrapper.closest('form');
    if (parentForm) {
      parentForm.addEventListener('submit', function (e) { e.preventDefault(); });
    }

    document.addEventListener('click', function (e) {
      if (!wrapper.contains(e.target)) {
        results.style.display = 'none';
      }
    });
  });

  function doSearch(query, resultsEl) {
    if (!index) {
      resultsEl.innerHTML = '<div class="ts-search-item ts-search-empty">Loading...</div>';
      resultsEl.style.display = 'block';
      return;
    }

    var terms = query.toLowerCase().split(/\s+/);
    var scored = [];

    for (var i = 0; i < index.length; i++) {
      var entry = index[i];
      var title = (entry.title || '').toLowerCase();
      var summary = (entry.summary || '').toLowerCase();
      var section = (entry.section || '').toLowerCase();
      var content = (entry.content || '').toLowerCase();
      var tags = (entry.tags || []).join(' ').toLowerCase();

      var score = 0;
      var matched = true;
      for (var t = 0; t < terms.length; t++) {
        var term = terms[t];
        var termScore = 0;
        if (title.indexOf(term) !== -1) termScore += 10;
        if (summary.indexOf(term) !== -1) termScore += 5;
        if (tags.indexOf(term) !== -1) termScore += 3;
        if (section.indexOf(term) !== -1) termScore += 2;
        if (content.indexOf(term) !== -1) termScore += 1;
        if (termScore === 0) { matched = false; break; }
        score += termScore;
      }

      if (matched && score > 0) {
        scored.push({ entry: entry, score: score });
      }
    }

    scored.sort(function (a, b) { return b.score - a.score; });
    var results = scored.slice(0, maxResults).map(function (s) { return s.entry; });
    renderResults(results, query, resultsEl);
  }

  function renderResults(results, query, resultsEl) {
    if (results.length === 0) {
      resultsEl.innerHTML = '<div class="ts-search-item ts-search-empty">No results for "' + escapeHtml(query) + '"</div>';
      resultsEl.style.display = 'block';
      return;
    }

    var html = '';
    for (var i = 0; i < results.length; i++) {
      var r = results[i];
      var section = r.section ? '<span class="ts-search-section">' + escapeHtml(r.section) + '</span>' : '';
      html += '<a href="' + escapeHtml(r.url) + '" class="ts-search-item">';
      html += '<div class="ts-search-title">' + escapeHtml(r.title) + section + '</div>';
      if (r.summary) {
        html += '<div class="ts-search-summary">' + escapeHtml(truncate(stripHtml(r.summary), 120)) + '</div>';
      }
      html += '</a>';
    }
    resultsEl.innerHTML = html;
    resultsEl.style.display = 'block';
  }

  function escapeHtml(text) {
    var div = document.createElement('div');
    div.appendChild(document.createTextNode(text));
    return div.innerHTML;
  }

  function stripHtml(html) {
    var div = document.createElement('div');
    div.innerHTML = html;
    return div.textContent || div.innerText || '';
  }

  function truncate(str, len) {
    if (str.length <= len) return str;
    return str.substring(0, len) + '...';
  }
})();

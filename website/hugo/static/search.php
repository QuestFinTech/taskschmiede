<?php
// Taskschmiede Documentation Search
// Server-side search with query logging for docs improvement.
//
// Copyright 2026 Quest Financial Technologies S.a r.l.-S., Luxembourg
// Licensed under the Apache License, Version 2.0

header('Content-Type: application/json; charset=utf-8');
header('X-Content-Type-Options: nosniff');

// Configuration
$indexFile = __DIR__ . '/search-index.json';
$logDir = '/var/log/taskschmiede-docs';
$logFile = $logDir . '/search.log';
$maxResults = 20;

// Get query
$query = isset($_GET['q']) ? trim($_GET['q']) : '';
if ($query === '') {
    echo json_encode(['results' => [], 'query' => '', 'total' => 0]);
    exit;
}

// Sanitize query for logging (remove control chars, limit length)
$safeQuery = preg_replace('/[\x00-\x1f\x7f]/', '', $query);
$safeQuery = mb_substr($safeQuery, 0, 200);

// Load search index
if (!file_exists($indexFile)) {
    http_response_code(500);
    echo json_encode(['error' => 'Search index not found']);
    exit;
}

$indexData = file_get_contents($indexFile);
$entries = json_decode($indexData, true);
if (!is_array($entries)) {
    http_response_code(500);
    echo json_encode(['error' => 'Invalid search index']);
    exit;
}

// Search: split query into terms, score each entry
$terms = preg_split('/\s+/', mb_strtolower($safeQuery));
$scored = [];

foreach ($entries as $entry) {
    $score = 0;
    $titleLower = mb_strtolower($entry['title'] ?? '');
    $summaryLower = mb_strtolower($entry['summary'] ?? '');
    $contentLower = mb_strtolower($entry['content'] ?? '');
    $sectionLower = mb_strtolower($entry['section'] ?? '');

    foreach ($terms as $term) {
        if ($term === '') continue;

        // Title match (highest weight)
        if (mb_strpos($titleLower, $term) !== false) {
            $score += 10;
            // Exact title match bonus
            if ($titleLower === $term) {
                $score += 5;
            }
        }

        // Summary match
        if (mb_strpos($summaryLower, $term) !== false) {
            $score += 5;
        }

        // Section match
        if (mb_strpos($sectionLower, $term) !== false) {
            $score += 2;
        }

        // Content match (lowest weight)
        if (mb_strpos($contentLower, $term) !== false) {
            $score += 1;
        }

        // Tag match
        if (isset($entry['tags']) && is_array($entry['tags'])) {
            foreach ($entry['tags'] as $tag) {
                if (mb_strpos(mb_strtolower($tag), $term) !== false) {
                    $score += 3;
                }
            }
        }
    }

    if ($score > 0) {
        $scored[] = [
            'title' => $entry['title'] ?? '',
            'summary' => $entry['summary'] ?? '',
            'url' => $entry['url'] ?? '',
            'section' => $entry['section'] ?? '',
            'score' => $score,
        ];
    }
}

// Sort by score descending
usort($scored, function ($a, $b) {
    return $b['score'] - $a['score'];
});

// Limit results
$results = array_slice($scored, 0, $maxResults);

// Remove score from output
$output = array_map(function ($r) {
    unset($r['score']);
    return $r;
}, $results);

// Log the query
$referer = isset($_SERVER['HTTP_REFERER']) ? $_SERVER['HTTP_REFERER'] : '-';
// Strip domain from referer to keep just the path
$refererPath = parse_url($referer, PHP_URL_PATH) ?? '-';
$ip = $_SERVER['HTTP_X_REAL_IP']
    ?? $_SERVER['HTTP_X_FORWARDED_FOR']
    ?? $_SERVER['REMOTE_ADDR']
    ?? '-';
// Take first IP if X-Forwarded-For contains multiple
if (strpos($ip, ',') !== false) {
    $ip = trim(explode(',', $ip)[0]);
}

$logLine = sprintf(
    "%s | q=\"%s\" | results=%d | referer=%s | ip=%s\n",
    gmdate('Y-m-d\TH:i:s\Z'),
    addcslashes($safeQuery, '"\\'),
    count($results),
    $refererPath,
    $ip
);

// Write log (create directory if needed, fail silently if not writable)
if (is_dir($logDir) || @mkdir($logDir, 0755, true)) {
    @file_put_contents($logFile, $logLine, FILE_APPEND | LOCK_EX);
}

// Return results
echo json_encode([
    'results' => $output,
    'query' => $safeQuery,
    'total' => count($scored),
]);

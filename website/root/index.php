<?php
/**
 * Taskschmiede - Visitor Analytics Router
 *
 * Logs visitor information to SQLite, then routes to the appropriate page.
 * Static assets (CSS, JS) are served directly by NGINX.
 *
 * Analytics DB location: configured via ANALYTICS_DB_PATH environment
 * variable, or defaults to /var/www/taskschmiede-analytics/analytics.db
 */

$dbPath = getenv('ANALYTICS_DB_PATH')
    ?: '/var/www/taskschmiede-analytics/analytics.db';

// --- Log the visit ---

try {
    $dbDir = dirname($dbPath);
    if (!is_dir($dbDir)) {
        mkdir($dbDir, 0750, true);
    }

    $db = new PDO('sqlite:' . $dbPath);
    $db->setAttribute(PDO::ATTR_ERRMODE, PDO::ERRMODE_EXCEPTION);
    $db->exec('PRAGMA journal_mode=WAL');

    $db->exec('CREATE TABLE IF NOT EXISTS visits (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        ts TEXT NOT NULL DEFAULT (datetime(\'now\')),
        ip TEXT NOT NULL,
        method TEXT NOT NULL,
        path TEXT NOT NULL,
        user_agent TEXT,
        referrer TEXT,
        accept_language TEXT,
        country TEXT,
        city TEXT
    )');

    $db->exec('CREATE INDEX IF NOT EXISTS idx_visits_ts ON visits(ts)');
    $db->exec('CREATE INDEX IF NOT EXISTS idx_visits_ip ON visits(ip)');

    // REMOTE_ADDR is set by NGINX fastcgi_params and cannot be spoofed.
    // X-Forwarded-For / X-Real-IP are client-controllable headers — only
    // trust them if a reverse proxy in front of NGINX sets them.
    $ip = $_SERVER['REMOTE_ADDR'] ?? '';

    // GeoIP lookup
    $country = null;
    $city = null;
    $geoDbPath = getenv('GEOIP_DB_PATH')
        ?: '/var/lib/GeoIP/GeoLite2-City.mmdb';

    if ($ip && file_exists($geoDbPath)) {
        try {
            $geo = new MaxMind\Db\Reader($geoDbPath);
            $record = $geo->get($ip);
            $geo->close();
            if ($record) {
                $country = $record['country']['iso_code'] ?? null;
                $city = $record['city']['names']['en'] ?? null;
            }
        } catch (Exception $e) {
            error_log('Taskschmiede GeoIP error: ' . $e->getMessage());
        }
    }

    $stmt = $db->prepare('INSERT INTO visits
        (ip, method, path, user_agent, referrer, accept_language, country, city)
        VALUES (:ip, :method, :path, :ua, :ref, :lang, :country, :city)');

    $stmt->execute([
        ':ip'      => $ip,
        ':method'  => $_SERVER['REQUEST_METHOD'] ?? 'GET',
        ':path'    => $_SERVER['REQUEST_URI'] ?? '/',
        ':ua'      => $_SERVER['HTTP_USER_AGENT'] ?? null,
        ':ref'     => $_SERVER['HTTP_REFERER'] ?? null,
        ':lang'    => $_SERVER['HTTP_ACCEPT_LANGUAGE'] ?? null,
        ':country' => $country,
        ':city'    => $city,
    ]);
} catch (Exception $e) {
    // Analytics failure must never break the page
    error_log('Taskschmiede analytics error: ' . $e->getMessage());
}

// --- Route to the requested page ---

$routes = [
    '/'                => '_home.page',
    '/features'        => '_features.page',
    '/about'           => '_about.page',
    '/maintenance'     => '_maintenance.page',
];

$path = parse_url($_SERVER['REQUEST_URI'], PHP_URL_PATH);
$path = rtrim($path, '/') ?: '/';

$pageFile = $routes[$path] ?? null;
if ($pageFile === null) {
    http_response_code(404);
    $pageFile = '_404.page';
}

include __DIR__ . '/' . $pageFile;

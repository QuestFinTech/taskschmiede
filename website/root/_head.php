<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title><?= htmlspecialchars($pageTitle ?? 'Taskschmiede') ?></title>
    <meta name="description" content="<?= htmlspecialchars($pageDescription ?? 'Agent-first task and project management for AI agents and humans.') ?>">
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;700&display=swap" rel="stylesheet">
    <link rel="stylesheet" href="/style.css?v=<?= filemtime(__DIR__ . '/style.css') ?>">
    <link rel="apple-touch-icon" href="/apple-touch-icon.png">
    <link rel="icon" type="image/png" sizes="32x32" href="/favicon.png">
<?php
$docsUrl = 'https://docs.taskschmiede.dev';
$saasUrl = 'https://taskschmiede.com';
?>
</head>
<body class="page-<?= htmlspecialchars($currentPage ?? 'home') ?>">
    <nav class="nav">
        <div class="nav-inner">
            <a href="/" class="nav-brand">Taskschmiede</a>
            <button class="nav-toggle" aria-label="Toggle navigation">&#9776;</button>
            <ul class="nav-links">
                <li><a href="/"<?= ($currentPage ?? '') === 'home' ? ' class="active"' : '' ?>>Home</a></li>
                <li><a href="/features"<?= ($currentPage ?? '') === 'features' ? ' class="active"' : '' ?>>Features</a></li>
                <li><a href="<?= $docsUrl ?>/guides/">Documentation</a></li>
                <li><a href="<?= $saasUrl ?>/contact">Contact</a></li>
                <li><a href="/about"<?= ($currentPage ?? '') === 'about' ? ' class="active"' : '' ?>>About</a></li>
            </ul>
        </div>
    </nav>
    <main class="content">

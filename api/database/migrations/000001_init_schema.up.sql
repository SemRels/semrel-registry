CREATE TABLE IF NOT EXISTS plugins (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) UNIQUE NOT NULL,
    description TEXT,
    author VARCHAR(255),
    category VARCHAR(50) NOT NULL,
    repository TEXT,
    license VARCHAR(50),
    tags TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS plugin_versions (
    id SERIAL PRIMARY KEY,
    plugin_id INT NOT NULL REFERENCES plugins(id) ON DELETE CASCADE,
    version VARCHAR(50) NOT NULL,
    release_date TIMESTAMP,
    changelog TEXT,
    download_url TEXT NOT NULL,
    prerelease BOOLEAN DEFAULT false,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(plugin_id, version)
);

CREATE TABLE IF NOT EXISTS plugin_checksums (
    id SERIAL PRIMARY KEY,
    version_id INT NOT NULL REFERENCES plugin_versions(id) ON DELETE CASCADE,
    platform VARCHAR(50) NOT NULL,
    algorithm VARCHAR(20) NOT NULL,
    hash VARCHAR(255) NOT NULL
);

CREATE TABLE logData (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    account TEXT NOT NULL,
    system TEXT NOT NULL,
    user TEXT NOT NULL,
    module TEXT NOT NULL,
    task TEXT NOT NULL,
    timestamp DATETIME NOT NULL,
    msg TEXT NOT NULL,
    level INTEGER NOT NULL,
    stack_trace TEXT
);


CREATE INDEX IF NOT EXISTS idx_account ON logData(account);
CREATE INDEX IF NOT EXISTS idx_system ON logData(system);
CREATE INDEX IF NOT EXISTS idx_user ON logData(user);
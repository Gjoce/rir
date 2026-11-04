CREATE TABLE IF NOT EXISTS "public"."sessions" (
    "token" TEXT NOT NULL,
    "data" bytea NOT NULL,
    "expiry" TIMESTAMPTZ NOT NULL,
    PRIMARY KEY ("token")
);

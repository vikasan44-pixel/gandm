-- PostgreSQL requires a newly-added enum value to be committed before it
-- can be used. The data/index changes therefore live in migration 000040.
ALTER TYPE offer_status ADD VALUE IF NOT EXISTS 'withdrawn';

-- Passwords for Docker Compose only. They are public on purpose: the local
-- database listens on the loopback address and holds no real data. No other
-- environment runs this file.
ALTER ROLE facility_owner PASSWORD 'facility_owner';
ALTER ROLE facility_runtime PASSWORD 'facility_runtime';
ALTER ROLE reservation_owner PASSWORD 'reservation_owner';
ALTER ROLE reservation_runtime PASSWORD 'reservation_runtime';
ALTER ROLE notification_owner PASSWORD 'notification_owner';
ALTER ROLE notification_runtime PASSWORD 'notification_runtime';

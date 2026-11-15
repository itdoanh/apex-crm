-- ==============================================================
--  APEX FINTECH — Khởi tạo database cho môi trường LOCAL (Docker)
--
--  Container `apex-postgres` chỉ tự tạo 1 database (POSTGRES_DB=apex_leads).
--  Script này tạo thêm `apex_leads_test` ⇒ local có ĐÚNG 2 DB, khớp với
--  server production (apex_leads) và server test (apex_leads_test).
--
--  ⚠️ File này chỉ chạy 1 LẦN ĐẦU, khi volume `apex_postgres_data` còn rỗng.
--     Nếu volume đã có dữ liệu → script KHÔNG chạy lại. Khi đó tạo DB thủ công:
--       docker exec -it apex-postgres psql -U postgres -c "CREATE DATABASE apex_leads_test;"
-- ==============================================================

SELECT 'CREATE DATABASE apex_leads_test'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'apex_leads_test')\gexec

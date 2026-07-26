\set ON_ERROR_STOP on

DO $verify$
DECLARE
  actual_roles text[];
  actual_edges text[];
  expected_roles constant text[] := ARRAY[
    'nexusrelay_cluster_admin|true|true|true|true|true',
    'nexusrelay_control_plane|true|false|false|false|true',
    'nexusrelay_control_plane_runtime|false|false|false|false|false',
    'nexusrelay_gateway|true|false|false|false|true',
    'nexusrelay_gateway_runtime|false|false|false|false|false',
    'nexusrelay_migration|true|false|false|false|false',
    'nexusrelay_schema_owner|false|false|false|false|false',
    'nexusrelay_security_definer_owner|false|false|false|false|false',
    'nexusrelay_worker|true|false|false|false|true',
    'nexusrelay_worker_runtime|false|false|false|false|false'
  ];
  expected_edges constant text[] := ARRAY[
    'nexusrelay_control_plane->nexusrelay_control_plane_runtime|false|true|false',
    'nexusrelay_gateway->nexusrelay_gateway_runtime|false|true|false',
    'nexusrelay_migration->nexusrelay_schema_owner|false|false|true',
    'nexusrelay_migration->nexusrelay_security_definer_owner|false|false|true',
    'nexusrelay_worker->nexusrelay_worker_runtime|false|true|false'
  ];
BEGIN
  IF current_setting('server_version_num')::integer < 180000
     OR current_setting('server_version_num')::integer >= 190000 THEN
    RAISE EXCEPTION 'NexusRelay role verification requires PostgreSQL 18';
  END IF;

  SELECT array_agg(
    rolname || '|' || rolcanlogin || '|' || rolsuper || '|' ||
    rolbypassrls || '|' || rolcreaterole || '|' || rolinherit
    ORDER BY rolname
  )
  INTO actual_roles
  FROM pg_roles
  WHERE rolname LIKE 'nexusrelay\_%' ESCAPE '\';

  IF actual_roles IS DISTINCT FROM expected_roles THEN
    RAISE EXCEPTION 'NexusRelay pg_roles inventory or attributes differ from the approved PostgreSQL 18 graph';
  END IF;

  SELECT array_agg(
    member.rolname || '->' || granted.rolname || '|' ||
    membership.admin_option || '|' || membership.inherit_option || '|' ||
    membership.set_option
    ORDER BY member.rolname, granted.rolname
  )
  INTO actual_edges
  FROM pg_auth_members AS membership
  JOIN pg_roles AS granted ON granted.oid = membership.roleid
  JOIN pg_roles AS member ON member.oid = membership.member
  WHERE member.rolname LIKE 'nexusrelay\_%' ESCAPE '\'
     OR granted.rolname LIKE 'nexusrelay\_%' ESCAPE '\';

  IF actual_edges IS DISTINCT FROM expected_edges THEN
    RAISE EXCEPTION 'NexusRelay pg_auth_members edges or options differ from the approved PostgreSQL 18 graph';
  END IF;
END
$verify$;

SELECT 'postgresql|' || current_setting('server_version')
UNION ALL
SELECT 'role|' || rolname || '|' || rolcanlogin || '|' || rolsuper || '|' ||
       rolbypassrls || '|' || rolcreaterole || '|' || rolinherit
FROM pg_roles
WHERE rolname LIKE 'nexusrelay\_%' ESCAPE '\'
UNION ALL
SELECT 'edge|' || member.rolname || '->' || granted.rolname || '|' ||
       membership.admin_option || '|' || membership.inherit_option || '|' ||
       membership.set_option
FROM pg_auth_members AS membership
JOIN pg_roles AS granted ON granted.oid = membership.roleid
JOIN pg_roles AS member ON member.oid = membership.member
WHERE member.rolname LIKE 'nexusrelay\_%' ESCAPE '\'
   OR granted.rolname LIKE 'nexusrelay\_%' ESCAPE '\'
ORDER BY 1;

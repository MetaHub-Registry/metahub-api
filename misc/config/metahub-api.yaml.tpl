minio:
  url: {{ MINIO_URL |default("minio:9000") }}
  region: {{ MINIO_REGION |default("us-east-1") }}
  user: {{ MINIO_USER |default("minio") }}
  password: {{ MINIO_PASSWD |default("miniominio") }}
  redirect: {{ MINIO_REDIRECT |default("false") }}
  ssl: {{ MINIO_SSL |default("false") }}
  manifests:
    bucket: {{ MINIO_BUCKET_MANIFESTS |default("manifests") }}
  blobs:
    bucket: {{ MINIO_BUCKET_BLOBS |default("blobs") }}
neo4j:
  dbname: {{ NEO4J_DB_NAME |default("neo4j") }}
  user: {{ NEO4J_USER |default("neo4j") }}
  password: {{ NEO4J_PASSWD |default("test") }}
  url: bolt://{{ NEO4J_HOST |default("neo4j") }}:{{ NEO_BOLT_PORT |default("7687") }}
frontend:
  redirect: {{ FRONTEND_REDIRECT | default("https://frontend.metahub-registry.org") }}
cors:
  enabled: {{ CORS_ENABLED |default("false")}}
logging:
  level: {{ LOGSTASH_URL |default("debug") }}{% if LOGSTASH_URL %}
  gelf: {{ LOGSTASH_URL }}:12201{% endif %}
{% if OAUTH_CLIENTID %}oauth:
  clientid: {{ OAUTH_CLIENTID }}
  secret: {{ OAUTH_SECRET }}
  state: {{ OAUTH_STATE |default("supersecretsalt") }}{% endif %}
{% if GODMODE_USER %}godmode:
  signing_key: "a-random-secret-key-at-least-32-characters"
  users:
    - username: {{ GODMODE_USER }}
      password: {{ GODMODE_PASS |default(GODMODE_USER) }}
      role: {{ GODMODE_ROLE |default("admin") }}{% endif %}

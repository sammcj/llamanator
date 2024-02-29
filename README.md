# Llamde

## Template examples

```json
{
  "templates": {
    "default": "{{.Query}} default template content",
    "lighting": "{{.Query}} lighting control template content"
  }
}
```

## Home assistant examples

Default template

```yaml
rest_command:
  call_default_template:
    url: "https://llamanator.local/template/default"
    method: POST
    headers:
      Authorization: "Bearer YOUR_SECRET_TOKEN"
    content_type: 'application/json'
    payload: '{"query": "Your query here"}'
```

## Curl

```bash
curl -X POST "http://localhost:28080/template/default" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_SECRET_TOKEN" \
  -d '{"query": "tell me a joke"}'
```

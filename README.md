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
    url: "http://my-golang-app/template/default"
    method: POST
    headers:
      Authorization: "Bearer YOUR_SECRET_TOKEN"
    content_type: 'application/json'
    payload: '{"query": "Your query here"}'
```

Lighting template

```yaml
rest_command:
  call_lighting_template:
    url: "http://my-golang-app/template/lighting"
    method: POST
    headers:
      Authorization: "Bearer YOUR_SECRET_TOKEN"
    content_type: 'application/json'
    payload: '{"query": "Your query for lighting"}'
```

## Curl

```bash
curl -X POST "http://localhost:28080/template/default" \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer YOUR_AUTH_TOKEN" \
     -d '{"query": "This is a test query.", "model": "custom-model-name"}'
```

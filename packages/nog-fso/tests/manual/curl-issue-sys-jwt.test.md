# Test: Issue FSO System JWT via Curl

- Purpose: Verify that API `fso/sysauth` works.

## Steps

### Issue JWT

Run:

```
echo '{
    "subuser": "foo+bar",
    "expiresIn": 36000,
    "aud": ["nogapp", "fso"],
    "san": ["DNS:files.example.com"],
    "scopes": [
        { "action": "bc/write", "name": "all" },
        { "action": "fso/session", "name": "files.example.com" },
        { "action": "fso/read-registry", "names": ["exreg"] },
        {
            "actions": [
                "fso/read-repo",
                "fso/confirm-repo"
            ],
            "paths": [
                "/example/*"
            ]
        }
    ]
}' \
| ddev bash -c '
    NOG_JWT=$(cat /nog/jwt/tokens/admin.jwt) &&
    curl -H "Authorization: Bearer ${NOG_JWT}" -X POST \
        -H "Content-Type: application/json" -d @- \
        "${NOG_API_URL}/v1/fso/sysauth"
' \
| jq .
```

Expected:

 - Should return a JWT.

### Verify permission denied

Run:

```
for scope in \
    '{ "action": "fso/invalid", "name": "/" }' \
    '{ "action": "fso/read-repo", "path": "invalid" }' \
; do
    echo "    TEST ${scope}"
    echo '{
        "subuser": "foo+bar",
        "expiresIn": 36000,
        "aud": ["fso"],
        "scopes": ['"${scope}"']
    }' \
    | ddev bash -c '
        NOG_JWT=$(cat /nog/jwt/tokens/admin.jwt) &&
        curl -H "Authorization: Bearer ${NOG_JWT}" -X POST \
            -H "Content-Type: application/json" -d @- \
            "${NOG_API_URL}/v1/fso/sysauth"
    ' \
    | jq .
done
```

Expected:

 - Should return "The effective user cannot use ...".

### Verify malformed scope

Run:

```
for scope in \
    '{ "name": "/foo" }' \
    '{ "action": "bc/read", "name": "foo", "names": ["foo"] }' \
    '{ "action": "fso/read-repo", "path": "/foo", "paths": ["/foo"] }' \
; do
    echo "    TEST ${scope}"
    echo '{
        "subuser": "foo+bar",
        "expiresIn": 36000,
        "aud": ["fso"],
        "scopes": ['"${scope}"']
    }' \
    | ddev bash -c '
        NOG_JWT=$(cat /nog/jwt/tokens/admin.jwt) &&
        curl -H "Authorization: Bearer ${NOG_JWT}" -X POST \
            -H "Content-Type: application/json" -d @- \
            "${NOG_API_URL}/v1/fso/sysauth"
    ' \
    | jq .
done
```

Expected:

 - Should return `Match.Error`.

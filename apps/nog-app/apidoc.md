<!--

This file contains the documentation for REST API routes that are implemented
at the app level.  Use h2 to describe routes to fit in the global table of
content.

XXX Consider moving the routes and the documentation to a package.

XXX Consider writing a script that automatically creates JSON based on real API
calls, as for other package (see nog-content).

-->

## Get Job Status

    GET /jobs/:jobId/status

**Request query params**

 - None

**Request body**

 - empty

**Response**

    Status: 200

```json
{
  "data": {
    "status": "completed"
  },
  "statusCode": 200
}
```

## Post Job Status

    POST /jobs/:jobId/status

**Request query params**

 - None

**Request body**

 - `retryId (Integer)`: Current retry ID of this job
 - `status (String)`: Either `'completed'`, `'running'` or `'failed'`
 - `reason (String)`: Optional, reason if status is `'failed'`

**Response**

    Status: 200

```json
{
  "data": {},
  "statusCode": 200
}
```

## Get Job Progress

    GET /jobs/:jobId/progress

**Request query params**

 - None

**Request body**

 - empty

**Response**

    Status: 200

```json
{
  "data": {
    "progress": {
      "completed": 1,
      "percent": 50,
      "total": 2
    }
  },
  "statusCode": 200
}
```

## Post Job Progress

    POST /jobs/:jobId/progress

**Request query params**

 - None

**Request body**

 - `retryId (Integer)`: Current retry ID of this job
 - `progress (Object)`: with
    - `completed (Integer)`: Number of completed tasks
    - `total (Integer)`: Number of total tasks

**Response**

    Status: 200

```json
{
  "data": {},
  "statusCode": 200
}
```

## Post Job Log

    POST /jobs/:jobId/log

**Request query params**

 - None

**Request body**

 - `retryId (Integer)`: Current retry ID of this job
 - `message (String)`: Message to log
 - `level (integer)`: Optional, verbose level

**Response**

    Status: 200

```json
{
  "data": {},
  "statusCode": 200
}
```

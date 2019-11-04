idversion_testdata = {
  "tests": [

    {
      "name": "idv0 commit",
      "type": "commit",
      "idversion": 0,
      "canonical": {
        "_id": "6bf5411358347a5a6cf5c6a129f169a2ba54043f",
        "authorDate": "2015-07-10T15:41:16Z",
        "authors": [ "unknown <unknown>" ],
        "commitDate": "2015-07-10T15:41:16Z",
        "committer": "unknown <unknown>",
        "message": "msg",
        "meta": { "foo": "bar" },
        "parents": [ "1111111111111111111111111111111111111111" ],
        "subject": "Initial commit",
        "tree": "1111111111111111111111111111111111111111"
      },
      "mongo": {
        "_id": "6bf5411358347a5a6cf5c6a129f169a2ba54043f",
        "authorDate": "2015-07-10T15:41:16Z",
        "authors": [ "unknown <unknown>" ],
        "commitDate": "2015-07-10T15:41:16Z",
        "committer": "unknown <unknown>",
        "message": "msg",
        "meta": {"more": [{ "key": "foo", "val": "bar" }]},
        "parents": [ "1111111111111111111111111111111111111111" ],
        "subject": "Initial commit",
        "tree": "1111111111111111111111111111111111111111"
      }
    },

    {
      "name": "idv1 commit UTC",
      "type": "commit",
      "idversion": 1,
      "canonical": {
        "_id": "ef6fc7997fcd151ab38d6be9c91a687e48aada5c",
        "authorDate": "2015-07-10T15:41:16+00:00",
        "authors": [ "unknown <unknown>" ],
        "commitDate": "2015-07-10T15:41:16+00:00",
        "committer": "unknown <unknown>",
        "message": "msg",
        "meta": { "foo": "bar" },
        "parents": [ "1111111111111111111111111111111111111111" ],
        "subject": "Initial commit",
        "tree": "1111111111111111111111111111111111111111"
      },
      "mongo": {
        "_id": "ef6fc7997fcd151ab38d6be9c91a687e48aada5c",
        "authorDate": "2015-07-10T15:41:16+00:00",
        "authors": [ "unknown <unknown>" ],
        "commitDate": "2015-07-10T15:41:16+00:00",
        "committer": "unknown <unknown>",
        "message": "msg",
        "meta": {"more": [{ "key": "foo", "val": "bar" }]},
        "parents": [ "1111111111111111111111111111111111111111" ],
        "subject": "Initial commit",
        "tree": "1111111111111111111111111111111111111111"
      }
    },

    {
      "name": "idv1 commit +-TZ",
      "type": "commit",
      "idversion": 1,
      "canonical": {
        "_id": "57ec88998f81bba0fca2c3c2fb1801d76eeb84b2",
        "authorDate": "2015-07-10T15:41:16+01:00",
        "authors": [ "unknown <unknown>" ],
        "commitDate": "2015-07-10T15:41:16-06:00",
        "committer": "unknown <unknown>",
        "message": "msg",
        "meta": { "foo": "bar" },
        "parents": [ "1111111111111111111111111111111111111111" ],
        "subject": "Initial commit",
        "tree": "1111111111111111111111111111111111111111"
      },
      "mongo": {
        "_id": "57ec88998f81bba0fca2c3c2fb1801d76eeb84b2",
        "authorDate": "2015-07-10T15:41:16+01:00",
        "authors": [ "unknown <unknown>" ],
        "commitDate": "2015-07-10T15:41:16-06:00",
        "committer": "unknown <unknown>",
        "message": "msg",
        "meta": {"more": [{ "key": "foo", "val": "bar" }]},
        "parents": [ "1111111111111111111111111111111111111111" ],
        "subject": "Initial commit",
        "tree": "1111111111111111111111111111111111111111"
      }
    },

    {
      "name": "idv1 commit, +-TZ, unicode",
      "type": "commit",
      "idversion": 1,
      "canonical": {
        "_id": "5b49720622049a13859aba4426e5966a92080306",
        "authorDate": "2015-07-10T15:41:16+01:00",
        "authors": [ "uñknøwn <unknown>" ],
        "commitDate": "2015-07-10T15:41:16-06:00",
        "committer": "uñknøwn <unknown>",
        "message": "mßg",
        "meta": { "foo": "bâr" },
        "parents": [ "1111111111111111111111111111111111111111" ],
        "subject": "Initial cømmit",
        "tree": "1111111111111111111111111111111111111111"
      },
      "mongo": {
        "_id": "5b49720622049a13859aba4426e5966a92080306",
        "authorDate": "2015-07-10T15:41:16+01:00",
        "authors": [ "uñknøwn <unknown>" ],
        "commitDate": "2015-07-10T15:41:16-06:00",
        "committer": "uñknøwn <unknown>",
        "message": "mßg",
        "meta": {"more": [{ "key": "foo", "val": "bâr" }]},
        "parents": [ "1111111111111111111111111111111111111111" ],
        "subject": "Initial cømmit",
        "tree": "1111111111111111111111111111111111111111"
      }
    },

    {
      "name": "idv0 object",
      "type": "object",
      "idversion": 0,
      "canonical": {
        "_id": "4a149137a7da974ef84ba2bae4720f88f2516cd1",
        "name": "baz",
        "blob": "2222222222222222222222222222222222222222",
        "meta": { "description": "desc", "content": "text", "foo": "bar" }
      },
      "mongo": {
        "_id": "4a149137a7da974ef84ba2bae4720f88f2516cd1",
        "name": "baz",
        "blob": "2222222222222222222222222222222222222222",
        "meta": {
          "description": "desc",
          "content": "text",
          "more": [{ "key": "foo", "val": "bar" }]
        }
      },
      "create": [
        {
          "_idversion": 0,
          "name": "baz",
          "blob": "2222222222222222222222222222222222222222",
          "meta": { "description": "desc", "content": "text", "foo": "bar" }
        },
        {
          "_idversion": 0,
          "name": "baz",
          "blob": "2222222222222222222222222222222222222222",
          "text": "text",
          "meta": { "description": "desc", "foo": "bar" }
        }
      ]
    },

    {
      "name": "idv0 object (all 0 blob)",
      "type": "object",
      "idversion": 0,
      "canonical": {
        "_id": "5410c6620564e2f5a5f5f7d84540cbe76ca36d49",
        "name": "baz",
        "blob": "0000000000000000000000000000000000000000",
        "meta": {}
      },
      "mongo": {
        "_id": "5410c6620564e2f5a5f5f7d84540cbe76ca36d49",
        "name": "baz",
        "blob": "0000000000000000000000000000000000000000",
        "meta": {"more": []}
      },
      "create": [
        {
          "_idversion": 0,
          "name": "baz",
          "blob": "0000000000000000000000000000000000000000",
          "meta": {}
        },
        {
          "_idversion": 0,
          "name": "baz",
          "blob": null,
          "meta": {}
        },
        {
          "_idversion": 0,
          "name": "baz",
          "blob": null,
          "text": null,
          "meta": {}
        }
      ]
    },

    {
      "name": "idv1 object (null blob, null text)",
      "type": "object",
      "idversion": 1,
      "canonical": {
        "_id": "b61122d6a882f2d977b3778d0108c103895d3b5c",
        "name": "baz",
        "text": null,
        "blob": null,
        "meta": { "description": "desc", "foo": "bar" }
      },
      "mongo": {
        "_id": "b61122d6a882f2d977b3778d0108c103895d3b5c",
        "name": "baz",
        "text": null,
        "blob": null,
        "meta": {
          "description": "desc",
          "more": [{ "key": "foo", "val": "bar" }]
        }
      },
      "create": [
        {
          "name": "baz",
          "text": null,
          "blob": null,
          "meta": { "description": "desc", "foo": "bar" }
        },
        {
          "name": "baz",
          "text": null,
          "blob": "0000000000000000000000000000000000000000",
          "meta": { "description": "desc", "foo": "bar" }
        }
      ]
    },

    {
      "name": "idv1 object (blob, null text)",
      "type": "object",
      "idversion": 1,
      "canonical": {
        "_id": "b2e174b9736957f0a60974affa34962332bc5405",
        "name": "baz",
        "text": null,
        "blob": "2222222222222222222222222222222222222222",
        "meta": { "description": "desc", "foo": "bar" }
      },
      "mongo": {
        "_id": "b2e174b9736957f0a60974affa34962332bc5405",
        "name": "baz",
        "text": null,
        "blob": "2222222222222222222222222222222222222222",
        "meta": {
          "description": "desc",
          "more": [{ "key": "foo", "val": "bar" }]
        }
      },
      "create": [
        {
          "name": "baz",
          "text": null,
          "blob": "2222222222222222222222222222222222222222",
          "meta": { "description": "desc", "foo": "bar" }
        },
        {
          "name": "baz",
          "blob": "2222222222222222222222222222222222222222",
          "meta": { "description": "desc", "foo": "bar" }
        }
      ]
    },

    {
      "name": "idv1 object (null blob, text)",
      "type": "object",
      "idversion": 1,
      "canonical": {
        "_id": "2acfa1266799d8a727d5e233a9543b3e33bd5e27",
        "name": "baz",
        "text": "Lorem ipsum dolor",
        "blob": null,
        "meta": { "description": "desc", "foo": "bar" }
      },
      "mongo": {
        "_id": "2acfa1266799d8a727d5e233a9543b3e33bd5e27",
        "name": "baz",
        "text": "Lorem ipsum dolor",
        "blob": null,
        "meta": {
          "description": "desc",
          "more": [{ "key": "foo", "val": "bar" }]
        }
      },
      "create": [
        {
          "name": "baz",
          "text": "Lorem ipsum dolor",
          "blob": null,
          "meta": { "description": "desc", "foo": "bar" }
        },
        {
          "name": "baz",
          "blob": null,
          "meta": {
            "content": "Lorem ipsum dolor",
            "description": "desc",
            "foo": "bar"
          }
        }
      ]
    },


    {
      "name": "idv1 object (null blob, unicode text and meta)",
      "type": "object",
      "idversion": 1,
      "canonical": {
        "_id": "c5542c993d4a5ee9bf98e7af92aad283d9609388",
        "name": "baz",
        "text": "Lorem îpsum dølør",
        "blob": null,
        "meta": { "description": "descrîption", "foo": "bør" }
      },
      "mongo": {
        "_id": "c5542c993d4a5ee9bf98e7af92aad283d9609388",
        "name": "baz",
        "text": "Lorem îpsum dølør",
        "blob": null,
        "meta": {
          "description": "descrîption",
          "more": [{ "key": "foo", "val": "bør" }]
        }
      },
      "create": [
        {
          "name": "baz",
          "text": "Lorem îpsum dølør",
          "blob": null,
          "meta": { "description": "descrîption", "foo": "bør" }
        }
      ]
    },

    {
      "name": "idv0 tree",
      "type": "tree",
      "idversion": 0,
      "canonical": {
        "_id": "2efbaf81709fe7005b6ed14e3c6ebbbb1aedc282",
        "name": "baz",
        "entries": [
          {
            "type": "object",
            "sha1": "3333333333333333333333333333333333333333"
          }
        ],
        "meta": { "description": "desc", "foo": "bar" }
      },
      "mongo": {
        "_id": "2efbaf81709fe7005b6ed14e3c6ebbbb1aedc282",
        "name": "baz",
        "entries": [
          {
            "type": "object",
            "sha1": "3333333333333333333333333333333333333333"
          }
        ],
        "meta": {
          "description": "desc",
          "more": [{ "key": "foo", "val": "bar" }]
        }
      }
    },

    {
      "name": "idv0 tree, unicode name and meta",
      "type": "tree",
      "idversion": 0,
      "canonical": {
        "_id": "c8c0b1b192e7389a283522e544ce6ef1f67bdf78",
        "name": "bøz",
        "entries": [
          {
            "type": "object",
            "sha1": "3333333333333333333333333333333333333333"
          }
        ],
        "meta": { "description": "désc", "foo": "bør" }
      },
      "mongo": {
        "_id": "c8c0b1b192e7389a283522e544ce6ef1f67bdf78",
        "name": "bøz",
        "entries": [
          {
            "type": "object",
            "sha1": "3333333333333333333333333333333333333333"
          }
        ],
        "meta": {
          "description": "désc",
          "more": [{ "key": "foo", "val": "bør" }]
        }
      }
    }

  ]
}

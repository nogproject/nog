#!/bin/bash
# vim: sw=4
set -o errexit -o nounset -o pipefail -o noglob

# See `backend/HACKING-fso.md`

repoId=$(
    nogfsoctl get repos exreg \
    | grep /example/nog/sys/fixed-md \
    | cut -d '"' -f 4
)
echo "repo: ${repoId}"

put() {
    nogfsoctl gitnog put-path-metadata \
        --author='tester <tester@example>' \
        --message='write testing metadata' \
        ${repoId} "$1"
}

put 'keywords.prop={
    "type": "Property",
    "id": "$nogScopeSymbolPropertyId",
    "mdns": "/sys/md/nog",
    "symbol": "keywords",
    "names": [
        "Keywords",
        "Topics"
    ],
    "nameTokens": [
        "keywords",
        "topics"
    ],
    "description": "Keywords can be used to associate search terms with any content.",
    "examples": [
        "cellular orientation"
    ],
    "suggestValues": "TypedItem",
    "suggestValuesParams": {
        "ofType": [
            "$knownThis"
        ]
    },
    "knownValues": [
        "amira",
        "bcpfs",
        "fluoromath",
        "hand",
        "release"
    ]
}'

put 'author.prop={
    "nog_fixed_md": {
        "type": "Property",
        "id": "$nogSymbolPropertyId",
        "symbol": "author",
        "suggestValues": "TypedItem",
        "suggestValuesParams": {
            "ofType": [
                "Q5",
                "$knownThis"
            ]
        },
        "knownValues": [
            "Steffen Prohaska",
            "Uli Homberg",
            "Marc Osterland"
        ]
    }
}'

# The Wikidata item 'human (Q5)`.
put 'human_q5.item={
    "nog_fixed_md": [
        {
            "type": "Item",
            "id": "Q5",
            "symbol": "human",
            "names": [
                "human",
                "person"
            ],
            "description": "An individual human being."
        }
    ]
}'

put 'sugns/keywords.prop={
    "op": "EnableProperty",
    "id": "$nogScopeSymbolPropertyId",
    "mdns": "/sys/md/nog",
    "symbol": "keywords",
    "sugns": "/sys/sug/g/visual"
}'

put 'sugns/doi.prop={
    "nog_fixed_suggestion_namespace": {
        "op": "EnableProperty",
        "id": "$nogSymbolPropertyId",
        "symbol": "doi",
        "mdns": "/sys/md/nog",
        "sugns": "/sys/sug/g/visual"
    }
}'

put 'sugns/multiple.props={
    "nog_fixed_suggestion_namespace": [
        {
            "op": "EnableProperty",
            "id": "$nogScopeSymbolPropertyId",
            "mdns": "/sys/md/g/visual",
            "symbol": "specimen_patient",
            "sugns": "/sys/sug/g/visual"
        },
        {
            "op": "EnableProperty",
            "id": "P2049",
            "mdns": "/sys/md/wikidata",
            "sugns": "/sys/sug/default"
        }
    ]
}'

put 'sugns/keywords.ptype={
    "op": "EnablePropertyType",
    "id": "$nogScopeSymbolPropertyId",
    "mdns": "/sys/md/nog",
    "symbol": "keywords",
    "sugns": "/sys/sug/g/visual",
    "suggestFromMdnss": [
        "/sys/md/wikidata",
        "/sys/md/nog",
        "/sys/md/g/visual"
    ]
}'

# Wikidata:
#
# - 'Q11573': `m`
# - 'Q175821': `um`
# - 'Q178674': `nm`
#
put 'width_p2049.prop={
  "type": "Property",
  "id": "P2049",
  "symbol": "width",
  "names": [
    "Width"
  ],
  "nameTokens": [
    "$tokensFromNames"
  ],
  "description": "width of an object",
  "examples": [
    "10 nm"
  ],
  "suggestValues": "Quantity",
  "suggestValuesParams": {
    "units": [
      "Q11573",
      "Q175821",
      "Q178674"
    ]
  }
}'

put 'length-units.items={
    "nog_fixed_md": [
        {
            "type": "Item",
            "id": "Q11573",
            "symbol": "m",
            "names": [
                "metre",
                "m",
                "meter",
                "meters",
                "metres"
            ],
            "description": "SI unit of length"
        },
        {
            "type": "Item",
            "id": "Q175821",
            "symbol": "um",
            "names": [
                "micrometre",
                "micrometer",
                "µm",
                "micron",
                "um"
            ],
            "description": "one millionth of a metre"
        },
        {
            "type": "Item",
            "id": "Q178674",
            "symbol": "nm",
            "names": [
                "nanometre",
                "nm",
                "nanometer"
            ],
            "description": "unit of length"
        }
    ]
}'

put 'width.sugns={
    "nog_fixed_suggestion_namespace": [
        {
            "op": "EnableProperty",
            "id": "P2049",
            "mdns": "/sys/md/wikidata",
            "sugns": "/sys/sug/default"
        },
        {
            "op": "EnablePropertyType",
            "id": "P2049",
            "sugns": "/sys/sug/default",
            "suggestFromMdnss": [
                "/sys/md/wikidata"
            ]
        }
    ]
}'

# Package `nog-plateview`

`nog-plateview` is a package to display data in a standardized plate format.
Modern microscopes use microplates with 96, 384 or even 1536 wells. This data
would be shown as nog-tree with 96 to 1536 folders. To avoid this, the data is
arranged and displayed in the format it was created during the experiment.

Per default the plateview displays a link to the sub-folder in each well, if
there is data. Otherwise the well is empty.
A tooltip shows the description of a well, e.g. the treatment.

If numeric results are stored in meta, these can be displayed as well for the 
whole plate. The respective value is shown and color-coded. Min/Max value can
be specified in meta, or a Autoscale button can be used to adapt the color 
coding to the underlying values.


## Meta Structure

To activate the plateview, use the following meta structure:

```json
{
  "plateView": {
    "description": "Some description...",
    "name": "some Name",
    "results": [
      "cellcount",
      "param1"
    ],
    "show": "name",
    "vmax": 300,
    "vmin": 100,
    "wells": 384
  }
}
```

Mandatory parameters:

 - name (String): Name of the plate
 - wells (Number): Number of wells, must be 96, 384 or 1536
 
Recommended parameters:

 - show (String): Name of the feature to be shown, Default is 'name'.
 - description (String): Short Description of the plate.
 
Optional parameters:

 - results ([String]): List of available result features.
 - vmin (Number): Minimum parameter for color-coding. Every value below will be shown white.
 - vmax (Number): Maximum parameter for color-coding. Every value above will be shown blue.

Each well is a sub-tree of the plate can have this meta structure:

```json
{
  "description": "1mM DL-PDMP",
  "results": {
    "cellcount": 42,
    "param1": 13.37
  }
}
```

Optional parameters:

 - description (String): Short description, e.g. compound and concentration.
 - results (Object): Actual results for the well with feature name and value.
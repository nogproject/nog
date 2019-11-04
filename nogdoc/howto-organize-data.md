# How to organize project data?

We intend to add dedicated tools to help maintain collections of experimental
data together with relevant metadata, such as experiment id, specimen id, or
processing step.  But we are not yet there.

For now, we suggest the following procedure to organize data:

 - Cooperating domain scientists upload data archives together with a text
   document that describes relevant information as described in
   [howto-upload](./howto-upload.md).
 - The responsible data scientist at ZIB uses the Python REST API wrapper (see
   tutorial) to get the data from the upload repo and organize it in
   a dedicated file repository.
 - Domain scientist or data scientist create separate analysis workspace
   repositories as needed, copy data from the file repository, and apply
   analysis programs.

The dedicated file repository may initially be organized using an ad-hoc naming
scheme.  We envision that data scientists will develop better schemes.  Here
are a few ideas:

 - Files could be organized systematically and automatically in a hierarchy of
   meta information (like study / condition / specimen / processing step).
 - The hierarchy could be evolved and automatically rearranged when the needs
   become clearer as the project evolves.
 - Meta fields could be used together with the search to quickly locate files.

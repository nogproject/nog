# Getting Started with the Nog Web UI

The Nog Web UI can be used to organize data and apply analysis programs.  This
tutorial shows how to do this.

<!-- toc -->

## General information on the views of Nog Web UI

The Nog Web UI provides two ways to organize data in repositories: file and
workspace repositories. The file repository can be used to collect data and to
build and organize the data in a project-specific hierarchy. A workspace
repository can be used to apply analysis programs.  For more details, read our
recommendations on [how to organize project data](./howto-organize-data.md).

The Nog Web UI provides different views to handle the data. We recommend to use
the *File View* to organize and structure the data in your repositories and the
*Workspace View* to organize and apply programs to a specified set of data; it
is only available for workspace repositories. The following sections describe
how to use them.

## How to use File View

The *File View* is available for all kinds of repositories. It lets you browse
the hierarchy of your repositories and repositories shared with you, and it
provides possibilities to modify your own repositories.

The *File View* follows the common concept of file browsers. You can select
single and multiple objects &mdash; the operations allowed for the selected
objects then appear in the button row at the top of the object list.

## How to use Workspace View for data analysis?

The *Workspace View* guides you through the workflow of applying an analysis
program to specific data. It is organized in 4 sections:

1. Input data: viewing and adding data to your datalist that you aim to
   process.
2. Programs: adding programs, setting up the program parameters and starting
   the programs.
3. Jobs: viewing the state of current and recent jobs
4. Results: viewing the results or the sets of results.

To see an example and the result of this tutorial, have a look at the
repository
[nog/example_photo-gallery_2015](/nog/example_photo-gallery_2015/workspace).

### Creating a workspace repository

- Go to the Home screen and click the button *New Repository*.
- Choose *Workspace* and type the repository name of `<your/repository>`, e.g.
  `example_photo-gallery_2015`.
- Click *Create Repository*. You will be directed to the *Workspace View* of
  the new repository containing the four sections mentioned above.

### Section Input Data

This section shows a list of the data in your repository and provides you the
possibility to add data to your `datalist` and to inspect them.

#### Adding data to a workspace repository

You can either upload data or copy data from another repository.

##### Uploading data from your computer

- Go to the section *Input Data* of `<your/repository>` and click the
  *Upload Files* button.
- Click *Choose files* and select the files you wish to upload.

##### Copying data from another repository

To copy data from another repository you can browse your and shared
repositories, or your can use the search on Nog. In both cases, you will browse
or search in a specific 'adding mode': a blue block
*Adding files to `<your/repository>`* shows up at the top of the screen and
directs you back to your workspace repository.

**Browsing for files:**

- Click the *Browse for files* button at the bottom of the section *Input
  Data*.  You will be directed to your Home screen showing the lists of
  repositories and the blue block of the 'adding mode'.
- Choose a repository displayed on your Home screen, e.g.
  [nog/example_photo-data_2015](/nog/example_photo-data_2015/files) and
  go to the folder `datalist`.
- Select the images you want to copy and choose *Add data to datalist in
  `<your/repository>`*.
- Click the arrow button of the blue block *Adding files to
  `<your/repository>`* at the top of the screen to go back to your workspace.

**Searching for files:**

- Click the *Search for data on nog* button at the bottom of the section *Input
  Data*.  You will be directed to the nog search showing the search field and
  the blue block of the 'adding mode'.
- Type words to search the repositories you have access to. For example, type
  'png' to get a list of all `.png` files
- Click the buttons *Add data to datalist in `<your/repository>`* next to the
  search items to add the file.
- Click the arrow button of the blue block *Adding files to
  `<your/repository>`* at the top of the screen to go back to your workspace.

#### Viewing the data of a workspace repository

The section *Input Data* displays a subset of the data in your `datalist`.
To get an overview you can inspect your data in a *View-only mode*:

- Click the *View all* button at the bottom of the section *Input Data*.  You
  will be directed to the *File View* of your `datalist` and the blue block
  of the *View-only mode* shows up at the top of the screen.
- You can browse your `datalist` and then go back to your workspace by clicking
  the arrow button of the blue block *View-only mode*.

### Section Programs

The program section shows the list of programs on the left and program specific
information on the right. It also provides the possibility to

- add programs from *Program Registries* that are shared with you,
- set up program parameters, and
- run the program.

#### Adding an analysis program to a workspace repository

There are two ways to add programs to your workspace. In both cases you will
browse or search in an 'adding mode' similar to the one of the section *Input
Data*.

**Browsing for programs**

- Click the *Browse for programs* button at the bottom of the section. You will
  be directed to your Home screen showing the lists of repositories and the
  blue block *Adding programs to `<your/repository>`*.
- Choose a program registry, e.g
  [nog/example_programs_2015](/nog/example_programs_2015/files) and go to
  the folder `programs`.
- Select the line of the program `photo-gallery` and click the button *Add to
  list of programs in `<your/repository>`*
- Click the arrow button of the blue block *Adding program to
  `<your/repository>`* at the top of the screen to go back to your workspace.
  The program `photo-gallery` now appears in the program section.

**Searching for programs**

- Click the *Search for programs on nog* button at the bottom of the section.
  You will be directed to nog search showing the search field and the blue
  block *Adding programs to `<your/repository>`*.
- Type words to search the repositories you have access to. For example, type
  '"path:programs/photo-gallery"'. The resulting list may show you multiple
  items, but an add button appears only for programs you can add.
- Click the button *Add program to list of programs in `<your/repository>`*
  next to the search item to add the program.
- Click the arrow button of the blue block *Adding files to
  `<your/repository>`* at the top of the screen to go back to your workspace.
  The program `photo-gallery` now appears in the program section.

#### Changing the parameters of an analysis program

The photo-gallery example has parameters to control the maximum height and
width of the thumbnails. You can change them as follows:

- The current parameters are displayed in the text field *Program Parameters
  (JSON)*. Change the parameters in the text field. You must use valid JSON.
- Click *Save Parameters*.

#### Setting multiple parameter sets for an analysis program

- Extend the default parameters by a list of dictionaries containing the
  varying parameters (example below). The program `photo-gallery`
  expects a list named `variants` and replaces the default parameters by the
  given parameters.
- Click *Save Parameters*.

```json
{
  "maxHeight": 200,
  "maxWidth": 200,
  "variants": [
    {
      "name": "default"
    },
    {},
    {
      "maxHeight": 299,
      "name": "large"
    },
    {
      "maxHeight": 100,
      "name": "small"
    },
    {
      "maxWidth": 150
    }
  ]
}
```

#### Applying an analysis program in a workspace repository

- Click the *Run* button to start the computation.
- Information on the started jobs and the results will then appear in the
  respective sections.

### Section Jobs

The job section provides the possibility to observe the progress of the latest
program-specific jobs.

To view information on all jobs click the *Show all* button and to clean up the
list of jobs click the *Delete all* button.

### Section Results

The result section shows the list of the program-specific results on the left
and the result report or the set of result reports on the right.

You can browse the resulting files in *view-only mode* by clicking the *Browse
results*  button and go back to your workspace by clicking the arrow button of
the blue block *View-only mode*.

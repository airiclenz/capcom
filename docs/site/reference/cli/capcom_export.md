---
title: "capcom export"
description: "Write the Issues and Notes as a static HTML site"
sidebar:
  order: 1
---

## capcom export

Write the Issues and Notes as a static HTML site

### Synopsis

Write the Issues and Notes as a static HTML site into a directory.

The site opens from the file system: index.html lists the Ready Issues, issues.html every Issue as a tree, closed and dropped ones included, and knowledge.html the Knowledge tree, with one page per Issue and per Note. It holds no JavaScript and no external resource.

The directory is created when missing. An export writes a .capcom-export marker into it, and the next export into that directory removes everything else in it first. A non-empty directory without the marker is refused, and so is a directory inside .capcom or one holding it.

The same Issues and Notes always give the same bytes. It prints the directory and the number of pages written.

```
capcom export --html <dir> [flags]
```

### Examples

```
  capcom export --html site          # write the site into ./site
  capcom export --html site --json   # {dir, pages} in a JSON envelope
```

### Options

```
  -h, --help          help for export
      --html string   the directory to write the HTML site into (required)
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom](capcom.md)	 - A git-native issue tracker and knowledge store for coding agents


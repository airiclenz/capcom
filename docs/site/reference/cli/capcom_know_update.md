---
title: "capcom know update"
description: "Change a Note's title, labels, refs, retirement or body"
sidebar:
  order: 1
---

## capcom know update

Change a Note's title, labels, refs, retirement or body

### Synopsis

Change one Note; only the flags given change anything, and at least one is required. --title, --add-label, --remove-label, --add-ref and --remove-ref edit its frontmatter; an --add-ref inside the git directory is refused, since a clone never has it, and one that does not resolve here, or that git ignores, is written with a warning, as capcom check fails on it. --retire marks it retired, so it and its subtree leave browsing, --unretire clears that, and --add-superseded-by names the Notes that replace it. --body-file replaces the whole body, --body-append and --body-append-file add to it, and --body-replace-old with --body-replace-new replaces text that occurs exactly once. --body-append and the --body-replace-old/--body-replace-new pair repeat: every pair applies in order, each against the body the pairs before it left, then every append in order, and when one pair fails nothing is written. It prints the Note's id, path and new etag.

```
capcom know update <id|alias|prefix> [flags]
```

### Examples

```
  capcom know update release-steps --add-label release --if-match <etag>   # etag from know show
  capcom know update adr-0012 --retire --add-superseded-by adr-0031   # replaced in whole
  capcom know update release-steps --body-append "Run the gate first."   # add to the body
  capcom know update release-steps --body-replace-old "v1" --body-replace-new "v2" --body-replace-old "draft" --body-replace-new "final"   # pairs apply in order
  capcom know update release-steps --body-file notes.md   # replace the whole body
```

### Options

```
      --add-label strings                  labels to add
      --add-ref stringArray                a path or path#symbol to add to the refs; repeat for more
      --add-superseded-by stringArray      the id, alias or prefix of a Note that replaces this one; repeat for more
      --body-append stringArray            text to append to the body, taken literally; repeat to append several in order
      --body-append-file string            file to append to the body; - reads stdin
      --body-file string                   file holding the whole new markdown body; - reads stdin
      --body-replace-new stringArray       text that replaces the --body-replace-old in the same position
      --body-replace-old stringArray       body text to replace; must occur exactly once; repeat with --body-replace-new for several pairs
  -h, --help                               help for update
      --if-match string                    only write when the file's etag is this one (see know show)
      --remove-label strings               labels to remove
      --remove-ref stringArray             a ref to remove; repeat for more
      --remove-superseded-by stringArray   the id, alias or prefix of a superseded-by entry to remove; repeat for more
      --retire                             mark the Note retired: it and its subtree leave browsing
      --title string                       new title
      --unretire                           clear the Note's retired mark
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom know](capcom_know.md)	 - Read, search, add, update, move, remove and verify Knowledge Notes


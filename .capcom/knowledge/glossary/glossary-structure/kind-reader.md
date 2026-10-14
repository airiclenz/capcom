---
title: "Kind reader: the one reader of a Tree kind's node files, and the verdict on whether one is usable"
---

The one reader of a **Tree** kind's node files: it reads a file, or decodes bytes already read, and
gives back the node, its etag, its **Aliases**, every finding that keeps the file out of the index,
and one verdict on whether the file is usable: it is when there is no finding at all.
_Avoid_: parser, loader, codec

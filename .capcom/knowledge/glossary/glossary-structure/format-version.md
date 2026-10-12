---
title: "Format version: the integer in config.yaml that names the file format a store is written in"
---

The integer a repository's capcom files declare in the `format:` key of `config.yaml`, saying which
file format they are written in; a missing key means 1. A binary refuses a store whose **Format
version** is above the highest it reads, and adding any key to the files raises it: format 2 added
**Related links**, and format 3 added the Note keys `retired` and `superseded-by`.
_Avoid_: schema version, store version, config version

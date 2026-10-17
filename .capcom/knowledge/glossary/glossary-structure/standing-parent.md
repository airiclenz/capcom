---
title: "Standing Parent: an Issue marked standing that only groups its children and is never Ready, claimed or closed"
---

A **Parent** whose issue file carries `standing: true`: it only groups its children and is never
itself the work. A **Standing Parent** is never **Ready**, and while it carries the mark it is
never claimed (moved into the `doing` category) nor **Closed**; `capcom check` reports one whose
**Status** is in the `doing` category or a closed one. `capcom create --standing` and
`capcom update --standing` set the mark, `capcom update --no-standing` clears it.
_Avoid_: container, epic, umbrella issue

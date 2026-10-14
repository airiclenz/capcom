package tree

import (
	"path/filepath"
	"slices"
	"strings"
)

// FactOrigin is where the file a FileFact describes comes from.
type FactOrigin int

// The origins of a file fact.
const (
	// WalkOrigin is a node file the walk kept: a candidate node of the tree.
	WalkOrigin FactOrigin = iota + 1
	// ClaimOnlyOrigin is a file the walk left out, a node file under an orphan directory or a file
	// the walk refused to read, whose id and aliases still claim names. It is never a node.
	ClaimOnlyOrigin
)

// ParseState is how far the file a FileFact describes was read for its aliases.
type ParseState int

// The parse states of a file fact.
const (
	// ParsedFile parses; its aliases are the ones it records.
	ParsedFile ParseState = iota + 1
	// UnparsedFile does not parse; its aliases are the ones a recovering read found, and every
	// name it claims is an unusable claim. Its aliases only claim: they are never mapped and
	// never conflict.
	UnparsedFile
	// UnreadFile was not read for its aliases, so it claims its id alone.
	UnreadFile
)

// FileFact is what DecideMembership knows of one file: everything it decides from, gathered by its
// caller.
type FileFact struct {
	// Key names the file: an opaque path key, unique among the facts. A caller passes the paths it
	// names files by, absolute or relative, in one spelling; ChildrenDir(Key) is the directory
	// beside the file, in the same spelling.
	Key string
	// ID is the file's id: its file name without the .md extension.
	ID string
	// ParentID is the id of the node file the file lies directly below, or "" at the top level.
	ParentID string
	// Dir is the directory that holds the file, in Key's spelling.
	Dir string
	// Aliases are the aliases the file claims, each one CheckAliases passed, in order.
	Aliases []string
	// Parse is how far the file was read for its aliases.
	Parse ParseState
	// IsUsable is the caller's verdict on the file's content: false for one that fails its kind's
	// validation, whose every name is an unusable claim.
	IsUsable bool
	// Origin is where the file comes from.
	Origin FactOrigin
}

// isUnusable reports whether every name the file claims is an unusable claim: a claim-only file, a
// file that does not parse, or one its caller found unusable. A walk file left out for a repeated
// id is one too; DecideMembership adds it.
func (f FileFact) isUnusable() bool {
	return f.Origin == ClaimOnlyOrigin || f.Parse == UnparsedFile || !f.IsUsable
}

// LeftOutReason is why DecideMembership leaves a walk file out of the tree.
type LeftOutReason int

// The reasons a walk file is left out.
const (
	// RepeatedIDFile is a file of an id two or more walk files share.
	RepeatedIDFile LeftOutReason = iota + 1
	// BelowRepeatedID is a file under the directory beside a file of a repeated id.
	BelowRepeatedID
)

// LeftOutFile is a walk file DecideMembership leaves out, and why.
type LeftOutFile struct {
	// Fact indexes the file's FileFact.
	Fact int
	// Reason is why it is left out.
	Reason LeftOutReason
}

// RepeatedIDDir is the directory beside a file of a repeated id: if it exists on disk, it is left
// out with its whole subtree. Whether it exists is the caller's to find out.
type RepeatedIDDir struct {
	// Path is ChildrenDir of the file's Key.
	Path string
	// ID is the repeated id.
	ID string
}

// Conflict is a name two files claim: the file Key, reported, and the file Other, which claims it
// earlier. For an UnusableRefusal, Other is "".
type Conflict struct {
	// Key is the file reported.
	Key string
	// Other is the other file that claims Name, or "".
	Other string
	// Name is the id or alias both claim.
	Name string
	// IsAlias reports whether Key claims Name as an alias rather than as its id.
	IsAlias bool
	// IsOtherAlias reports whether Other claims Name as an alias rather than as its id.
	IsOtherAlias bool
}

// NameClaim is one file's claim on a name.
type NameClaim struct {
	// Fact indexes the claiming file's FileFact.
	Fact int
	// IsAlias reports whether the file claims the name as an alias rather than as its id.
	IsAlias bool
}

// RefusalReason is why a name must not be resolved. Its zero value refuses nothing.
type RefusalReason int

// The reasons a name is refused.
const (
	// DuplicateRefusal is a name two files claim, or one that stands for a repeated id; the
	// verdict's Conflict names two files.
	DuplicateRefusal RefusalReason = iota + 1
	// UnusableRefusal is a name an unusable file alone claims, or a walk file left out for a
	// repeated id; the verdict's Conflict names it in Key.
	UnusableRefusal
)

// NameVerdict is what DecideMembership finds of one name.
type NameVerdict struct {
	// Claims are the files that claim the name, by their id or an alias, ordered by the claiming
	// file's id, then key, an id claim before alias claims.
	Claims []NameClaim
	// Refusal is why the name must not be resolved; zero when nothing refuses it.
	Refusal RefusalReason
	// Conflict is the conflict that refuses the name; zero when Refusal is.
	Conflict Conflict
}

// Membership is DecideMembership's verdict on a set of files: which are nodes of the tree, which
// are left out and why, and what every name stands for. Facts are referred to by their index.
type Membership struct {
	// Kept index the walk files that are nodes of the tree, ordered by id, then key.
	Kept []int
	// LeftOut are the walk files left out for a repeated id, ordered by id, then key.
	LeftOut []LeftOutFile
	// RepeatedIDDirs are the directories beside every file of a repeated id, in the files' order.
	RepeatedIDDirs []RepeatedIDDir
	// IDConflicts has one conflict per file of a repeated id after the first, naming the first in
	// Other, ordered by id, then key.
	IDConflicts []Conflict
	// AliasConflicts has one conflict per alias a walk file claims that a kept node has as its id,
	// that another kept node maps, or that an earlier left-out file claims: the kept nodes' in
	// order, then the left-out files'.
	AliasConflicts []Conflict
	// Aliases maps each alias a kept node answers to to its fact.
	Aliases map[string]int
	// Names maps every name a file claims, by its id or an alias, to its verdict.
	Names map[string]NameVerdict
}

// repeatedID is an id two or more walk files share: the first of its files by key, and the
// conflict of its second file, which names both.
type repeatedID struct {
	first    int
	conflict Conflict
}

// repeatedIDDirConflict is a directory beside a file of a repeated id, with that id's conflict.
type repeatedIDDirConflict struct {
	path     string
	conflict Conflict
}

// decider gathers the verdicts of one DecideMembership.
type decider struct {
	facts     []FileFact
	verdict   Membership
	keptByID  map[string]int
	refusals  map[string]Conflict
	unusables map[string]bool
}

// DecideMembership is the membership function of the tree (ADR 0001, ADR 0002): it decides, from
// facts alone and with no I/O, which walk files are nodes of the tree, which are left out and why,
// and every name's claims and refusal. facts may come in any order; they are taken by id, then key.
//
// Every walk file of an id two or more walk files share is left out, the first file of the id
// named in Other of one conflict per later file; so is every walk file under the directory beside
// a file of such an id, at any depth, whose names stand for the innermost such id. The rest are
// kept. A kept node's alias that another kept node has as its id or maps is a conflict; so is a
// left-out file's alias a kept node claims or an earlier left-out file lists. Each such name is
// refused with its first conflict. An alias the first file of a repeated id lists, or that a file
// under one of its directories lists, is refused with that id's conflict; one only a later file of
// the id lists is refused as below.
//
// A claim-only file is never kept and never leaves another file out. The aliases of a file that
// does not parse are never mapped and never a conflict. Every name a claim-only, unparsed,
// unusable or left-out file claims and nothing else refuses is refused: as a duplicate naming the
// first other file that claims it, or, when it alone claims the name, as an unusable claim.
func DecideMembership(facts []FileFact) Membership {
	d := &decider{
		facts:     facts,
		verdict:   Membership{Aliases: map[string]int{}, Names: map[string]NameVerdict{}},
		keptByID:  map[string]int{},
		refusals:  map[string]Conflict{},
		unusables: map[string]bool{},
	}
	order := d.sortedFacts()
	var walked []int
	for _, i := range order {
		if facts[i].Origin == WalkOrigin {
			walked = append(walked, i)
		}
	}

	repeated := d.repeatedIDs(walked)
	dirs := d.repeatedIDDirs(walked, repeated)
	firstFileOf := d.place(walked, repeated, dirs)
	d.mapKeptAliases()
	d.checkLeftOutAliases(firstFileOf)
	d.collectClaims(order)
	d.refuseUnusableClaims(order)
	d.recordRefusals()
	return d.verdict
}

// sortedFacts returns the indexes of the facts ordered by id, then key.
func (d *decider) sortedFacts() []int {
	order := make([]int, len(d.facts))
	for i := range order {
		order[i] = i
	}
	slices.SortFunc(order, func(a, b int) int {
		if c := strings.Compare(d.facts[a].ID, d.facts[b].ID); c != 0 {
			return c
		}
		return strings.Compare(d.facts[a].Key, d.facts[b].Key)
	})
	return order
}

// repeatedIDs finds the ids two or more of walked share. It adds one conflict per file of such an
// id after the first, naming the first in Other, and refuses the id with the first such conflict.
func (d *decider) repeatedIDs(walked []int) map[string]*repeatedID {
	repeated := map[string]*repeatedID{}
	firsts := map[string]int{}
	for _, i := range walked {
		fact := d.facts[i]
		first, isSeen := firsts[fact.ID]
		if !isSeen {
			firsts[fact.ID] = i
			continue
		}
		conflict := Conflict{Key: fact.Key, Other: d.facts[first].Key, Name: fact.ID}
		d.verdict.IDConflicts = append(d.verdict.IDConflicts, conflict)
		d.refuse(fact.ID, conflict)
		if repeated[fact.ID] == nil {
			repeated[fact.ID] = &repeatedID{first: first, conflict: conflict}
		}
	}
	return repeated
}

// repeatedIDDirs returns the directory beside every file of walked whose id is repeated, with that
// id's conflict, and records each among the verdict's RepeatedIDDirs.
func (d *decider) repeatedIDDirs(
	walked []int,
	repeated map[string]*repeatedID,
) []repeatedIDDirConflict {
	var dirs []repeatedIDDirConflict
	for _, i := range walked {
		fact := d.facts[i]
		id := repeated[fact.ID]
		if id == nil {
			continue
		}
		path := ChildrenDir(fact.Key)
		dirs = append(dirs, repeatedIDDirConflict{path: path, conflict: id.conflict})
		dir := RepeatedIDDir{Path: path, ID: fact.ID}
		d.verdict.RepeatedIDDirs = append(d.verdict.RepeatedIDDirs, dir)
	}
	return dirs
}

// place keeps every file of walked that has no repeated id and lies under no directory of dirs,
// and leaves the rest out; a file under one of dirs has its id refused with the conflict of the
// innermost. It returns the conflict a left-out file's aliases stand for, by fact: the repeated
// id's for the first file of the id and for a file under one of dirs; a later file of the id has
// none.
func (d *decider) place(
	walked []int,
	repeated map[string]*repeatedID,
	dirs []repeatedIDDirConflict,
) map[int]Conflict {
	standsFor := map[int]Conflict{}
	for _, i := range walked {
		fact := d.facts[i]
		if id := repeated[fact.ID]; id != nil {
			if id.first == i {
				standsFor[i] = id.conflict
			}
			d.verdict.LeftOut = append(d.verdict.LeftOut, LeftOutFile{Fact: i, Reason: RepeatedIDFile})
			continue
		}
		if dir := enclosingDir(dirs, fact.Dir); dir != nil {
			d.refuse(fact.ID, dir.conflict)
			standsFor[i] = dir.conflict
			d.verdict.LeftOut = append(d.verdict.LeftOut, LeftOutFile{Fact: i, Reason: BelowRepeatedID})
			continue
		}
		d.keptByID[fact.ID] = i
		d.verdict.Kept = append(d.verdict.Kept, i)
	}
	return standsFor
}

// enclosingDir returns the innermost of dirs that the directory dir is or lies under, or nil when
// it is none of them and lies under none.
func enclosingDir(dirs []repeatedIDDirConflict, dir string) *repeatedIDDirConflict {
	var enclosing *repeatedIDDirConflict
	for i := range dirs {
		candidate := &dirs[i]
		if !isWithin(dir, candidate.path) {
			continue
		}
		if enclosing == nil || len(candidate.path) > len(enclosing.path) {
			enclosing = candidate
		}
	}
	return enclosing
}

// isWithin reports whether the directory dir is ancestor or lies under it, by either separator: a
// caller's keys may be slash-separated or use the OS separator.
func isWithin(dir string, ancestor string) bool {
	rest, isPrefix := strings.CutPrefix(dir, ancestor)
	if !isPrefix {
		return false
	}
	return rest == "" || rest[0] == '/' || rest[0] == filepath.Separator
}

// mapKeptAliases maps every alias of each kept node that parses, in order, to its fact, unless a
// kept node has it as its id or an earlier one maps it: that is a conflict, and the alias is
// refused.
func (d *decider) mapKeptAliases() {
	for _, i := range d.verdict.Kept {
		fact := d.facts[i]
		for _, alias := range mappedAliases(fact) {
			if conflict, isTaken := d.keptClaim(fact.Key, alias); isTaken {
				d.verdict.AliasConflicts = append(d.verdict.AliasConflicts, conflict)
				d.refuse(alias, conflict)
				continue
			}
			d.verdict.Aliases[alias] = i
		}
	}
}

// checkLeftOutAliases checks the aliases of every left-out file that parses, in order, once the
// kept nodes' aliases are mapped, mapping none. An alias a kept node has as its id or maps, or that an earlier
// left-out file lists, is a conflict and refused. Any other alias of a file that stands for a
// repeated id, by standsFor, is refused with that id's conflict.
func (d *decider) checkLeftOutAliases(standsFor map[int]Conflict) {
	owners := map[string]string{}
	for _, file := range d.verdict.LeftOut {
		fact := d.facts[file.Fact]
		for _, alias := range mappedAliases(fact) {
			conflict, isTaken := d.keptClaim(fact.Key, alias)
			if other, isListed := owners[alias]; !isTaken && isListed {
				conflict = Conflict{Key: fact.Key, Other: other, Name: alias, IsAlias: true, IsOtherAlias: true}
				isTaken = true
			}
			if isTaken {
				d.verdict.AliasConflicts = append(d.verdict.AliasConflicts, conflict)
				d.refuse(alias, conflict)
				continue
			}
			owners[alias] = fact.Key
			if repeated, isStanding := standsFor[file.Fact]; isStanding {
				d.refuse(alias, repeated)
			}
		}
	}
}

// mappedAliases returns the aliases of fact that are mapped and checked for conflicts: all of them,
// unless the file does not parse, whose recovered aliases only claim.
func mappedAliases(fact FileFact) []string {
	if fact.Parse == UnparsedFile {
		return nil
	}
	return fact.Aliases
}

// keptClaim returns the conflict of the file key listing alias when a kept node has it as its id or
// maps it, naming that node in Other.
func (d *decider) keptClaim(key string, alias string) (Conflict, bool) {
	if owner, isTaken := d.keptByID[alias]; isTaken {
		return Conflict{Key: key, Other: d.facts[owner].Key, Name: alias, IsAlias: true}, true
	}
	if owner, isTaken := d.verdict.Aliases[alias]; isTaken {
		return Conflict{
			Key:          key,
			Other:        d.facts[owner].Key,
			Name:         alias,
			IsAlias:      true,
			IsOtherAlias: true,
		}, true
	}
	return Conflict{}, false
}

// collectClaims records every file's claims, in order: its id, then each alias.
func (d *decider) collectClaims(order []int) {
	for _, i := range order {
		fact := d.facts[i]
		d.addClaim(fact.ID, NameClaim{Fact: i})
		for _, alias := range fact.Aliases {
			d.addClaim(alias, NameClaim{Fact: i, IsAlias: true})
		}
	}
}

// addClaim adds claim to the verdict of name.
func (d *decider) addClaim(name string, claim NameClaim) {
	verdict := d.verdict.Names[name]
	verdict.Claims = append(verdict.Claims, claim)
	d.verdict.Names[name] = verdict
}

// refuseUnusableClaims refuses every name an unusable or left-out file claims that nothing refuses
// yet: as a duplicate naming the first other file that claims it, or as an unusable claim when no
// other file does.
func (d *decider) refuseUnusableClaims(order []int) {
	isLeftOut := map[int]bool{}
	for _, file := range d.verdict.LeftOut {
		isLeftOut[file.Fact] = true
	}
	for _, i := range order {
		fact := d.facts[i]
		if !fact.isUnusable() && !isLeftOut[i] {
			continue
		}
		d.refuseUnusableClaim(i, fact.ID, false)
		for _, alias := range fact.Aliases {
			d.refuseUnusableClaim(i, alias, true)
		}
	}
}

// refuseUnusableClaim refuses name, which the unusable file i claims, isAlias telling how, unless
// something refuses it already.
func (d *decider) refuseUnusableClaim(i int, name string, isAlias bool) {
	if _, isRefused := d.refusals[name]; isRefused {
		return
	}
	conflict := Conflict{Key: d.facts[i].Key, Name: name, IsAlias: isAlias}
	for _, claim := range d.verdict.Names[name].Claims {
		if claim.Fact == i {
			continue
		}
		conflict.Other = d.facts[claim.Fact].Key
		conflict.IsOtherAlias = claim.IsAlias
		break
	}
	if conflict.Other == "" {
		d.unusables[name] = true
	}
	d.refusals[name] = conflict
}

// refuse refuses name with conflict, unless an earlier conflict already refuses it.
func (d *decider) refuse(name string, conflict Conflict) {
	if _, isRefused := d.refusals[name]; !isRefused {
		d.refusals[name] = conflict
	}
}

// recordRefusals writes every refusal into the verdict of its name.
func (d *decider) recordRefusals() {
	for name, conflict := range d.refusals {
		verdict := d.verdict.Names[name]
		verdict.Refusal = DuplicateRefusal
		if d.unusables[name] {
			verdict.Refusal = UnusableRefusal
		}
		verdict.Conflict = conflict
		d.verdict.Names[name] = verdict
	}
}

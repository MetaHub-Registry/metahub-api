// Seed for the quickstart graph.
//
// Without a profile, pushes are rejected with
//   "failed to authorize push for profile 'generic'"
// because profile resolution has nothing to resolve to. One profile with a
// permissive rule is enough to push and pull; richer collections come from
// metahub-community-edition.
MERGE (p:Profile {label: 'generic'})
SET p.name = 'generic',
    p.tag  = 'generic',
    p.rule = 'os:linux=true && cpu:noop=true';

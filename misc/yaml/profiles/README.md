# Profiles

This is a dummy design for a profile, or a custom setup that says "do this action
on these particular events." We will likely want to iterate on these. The current events are
described in the main [README](../README.md) and development is written about here.

## Events

 - Manifest list GET (post)
 - Pre manifest get

## Actions

### Filter

A filter always has to happen on the response (post) and should be provided
in a "jq like" reference on the object. I first created this design:

```yaml
profile:
  name: library/skylake:latest
  event: pre-manifest-list-get
  select: .manifests .[] .platform .features
  values:
    - cpu:skylake
```

But realized the yaml structure would need to be flexible to some unknown number
of actions, each with a name and some interface of parameters.

```yaml
profile:
  name: library/skylake:latest
  event: pre-manifest-list-get
  actions:
    - name: manifest-get-filter
      args:
        select: .manifests .[] .platform .features
        value: cpu:skylake
```

E.g., it will be up to the action to parse it's own parameters. The action will
have a unique name "manifest-get-filter" it is registered to.

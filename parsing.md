# Compose file parsing

This document describes the logic parsing and merging compose file 
and overrides.

## Phase 1: parse yaml document

Yaml document is parsed using de-facto standard [go-yaml](https://github.com/go-yaml/yaml)
library. This one manages anchors and aliases, which are only supported within
a yaml document (an override can't refer to another compose file anchor)

## Phase 2: key conversion

Yaml allows mapping keys to be any type, but compose only uses strings for simplicity.
A conversion is applied on the yaml tree for all mapping keys to become strings,
even the parser could have parsed those are numbers or booleans

```yaml
services:
  true:
    ...
```
is converted to :
```yaml
services:
  "true":
    ...
```

# Phase 3: interpolation

Compose supports a bash-style syntax to allow variables to be set on yaml values.
Interpolation is responsible to resolve those into actual values based on variables
defined as "environment" during the parsing.

```yaml
services:
  foo:
    image: "foo:${TAG}"
```
is converted to :
```yaml
services:
  foo:
    image: "foo:1.2.3"
```

Interpolation takes place early and on a per-document basis, so that the yaml
tree can be validated by json schema. If interpolation isn't applied validation
could fail as Compose specification JSON schema require some nodes to be a boolean
or number

# Phase 4: empty nodes

JSON doesn't consider empty and null to be equivalent, so does the JSON-schema.
But go-yaml parser make them both a `nil` value, so we need to patch the yaml tree
accordingly.

# Phase 5: validation

Resulting yaml tree is validated against the Compose specification JSON schema

# Phase 6: extends

A service can be defined based on another one, with the ability to override some
attributes for local usage. This is the role of the `extends` attribute.

Extended service yaml definition is cloned into a plain new yaml subtree then
the local service definition is merged as an override. This includes support
for `!reset` to remove an element from original service definition.

# Phase 7: merge overrides

If loaded document is an override, the yaml tree is merged with the one from 
main compose file. `!reset` can be used to remove elements.
The merge logic generally is "_append to lists, replace in mapping_" with a 
few exceptions:
- shell commands always are replaced by an override
- `options` is only merged if both file declare the same `driver`, otherwise 
  the override fully replaces the original.
- Attributes which can be expressed both as a mapping and a sequence are converted
  so that merge can apply on equivalent data structures.

# Phase 8: enforce unicity

While modeled as a list, some attributes actually require some unicity to be 
applied. Volume mount definition for a service typically must be unique
regarding the target mount path. As such attribute can be defined as a single
string and set by a variable, we have to apply the "_append to list_" merge
strategy then check for unicity.

# Phase 9: transform into canonical representation

Compose specification allows many attribute to have both a "short" and a "long"
syntax. It also supports use of single string or list of strings for some
repeatable attributes. Some attributes can be declared both as a list of
key=value strings or as a yaml mapping.

During loading, all those attributes are transformed into canonical 
representation, so that we get a single format that will match to go structs
for binding.

# Phase 10: extensions

Extension (`x-*` attributes) can be used in any place in the yaml document.
To make unmarshalling easier, parsing move them all into a custom `#extension`
attribute. This hack is very specific to the go binding.

# Phase 11: validation

During the loading process, some logical rules are checked. But some involved
relations between exclusive attributes, and must be checked as a dedicated phase.

A typical example is the use of `external` in a resource definition. As such a 
resource is not managed by Compose, having some resource creation attributes set
must result into an error being reported to the user

```yaml
networks:
  foo:
    external: true
    driver: macvlan # This will trigger an error, as external network should not have any resource creation parameter set 
```

# Phase 12: relative paths

Compose allows paths to be set relative to the project directory. Those get resolved
into absolute paths during this phase. This involves a few corner cases, as
- path might denote a local file, or a remote (docker host) path, with windows vs unix
  filesystem syntax conflicts
- some attributes are not modeled by the Compose specification and still are paths, like
  bind mount options set to the local volume driver

```yaml
volumes:
  data:
    driver: local
    driver_opts:
      type: 'none'
      o: 'bind'
      device: './data' # such a relative path must be resolved
```

# Phase 13: go binding

Eventually, the yaml tree can be unmarshalled into go structs. We rely on
[mapstructure](https://github.com/mitchellh/mapstructure) library for this purpose.
Decoder is configured so that custom decode function can be defined by target type, 
allowing type conversions. For example, byte units (`640k`) and durations set in yaml
as plain string are actually modeled in go types as `int64`.




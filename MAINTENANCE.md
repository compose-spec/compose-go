# Maintenance

Compose-go library has to be kept up-to-date with approved changes in [Compose-spec](https://github.com/compose-spec/compose-spec).
This typically require, as we define new attributes to be added to the spec

1. Update `schema` to latest version from compose-spec   
1. Create the matching struct/field in `types`  
1. Create the matching `CheckXX` method in `compatibility`
1. If new attribute replaces a legacy one we want to deprecate, create the adequate logic in `normalize.go`
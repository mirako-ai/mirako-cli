# Mirako CLI

### CLI roadmap:
- [ ] interactive launch
- [ ] interactive terminate
- [ ] interactive list
- [ ] avatar list
- [ ] avatar view

# Codegen on REST models
The codegen is using oapi-codegen to generate models from the Mirako REST API OpenAPI spec. However, since oapi-codegen does not support v3.1 of the OpenAPI spec, and from Mirako REST API v3.1 is the lowest version that could be used, currently we handcraft the models until the support of v3.1 goes live.

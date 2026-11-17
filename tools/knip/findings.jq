# One finding per line from `knip --reporter json`. $dir is the directory that
# knip ran in, relative to the repository, with a trailing slash.
def describe($type; $name):
  {
    files: "the file is not imported from any entry point",
    dependencies: "dependency \($name) is not used",
    devDependencies: "devDependency \($name) is not used",
    optionalPeerDependencies: "optional peer dependency \($name) is not used",
    unlisted: "\($name) is used but not listed in package.json",
    binaries: "binary \($name) is used but no dependency provides it",
    unresolved: "import \($name) cannot be resolved",
    exports: "export \($name) is not imported anywhere",
    types: "exported type \($name) is not imported anywhere",
    nsExports: "export \($name) is only used through a namespace",
    nsTypes: "exported type \($name) is only used through a namespace",
    enumMembers: "enum member \($name) is not used",
    namespaceMembers: "namespace member \($name) is not used",
    duplicates: "\($name) export the same value"
  }[$type] // "\($type): \($name)";

.issues[] | .file as $file
| to_entries[] | select(.value | type == "array") | .key as $type
| .value[]
| (if type == "array" then {name: (map(.name) | join(", ")), line: .[0].line} else . end) as $issue
| "\($dir)\($file):\($issue.line // 1): [knip/\($type)] \(describe($type; $issue.name))"

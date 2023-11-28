# Inox

<img src="https://avatars.githubusercontent.com/u/122291844?s=200&v=4" alt="a shield"></img>

🛡️ The Inox programming language is your **shield** against complexity.

Inox is released as a **single binary** that contains all you need to do
full-stack development.\
Inox deeply integrates with its built-in database engine, testing engine and
HTTP server.

_Dead simple config. Zero boilerplate. Secure by Default._

**Main Dev Features**

- [XML Expressions (HTML)](#xml-expressions)
- [HTTP Server - Filesystem Routing](#http-server---filesystem-routing)
- [Built-in Database](#built-in-database)
- [Project Server (LSP)](#project-server-lsp)
- [Virtual Filesystems](#virtual-filesystems)
- [Advanced Testing Engine](#advanced-testing-engine)
- [Built-in Browser Automation](#built-in-browser-automation)

**Security Features**

- [Permission system](#permission-system)
  - [Required permissions](#required-permissions)
  - [Isolation of dependencies](#isolation-of-dependencies)
  - [Process-Level Access Control](#process-level-access-control)
  - [Dropping permissions](#dropping-permissions)
- [DoS Mitigation (WIP)](#dos-mitigation)
- [Sensitive Data Protection](#sensitive-data-protection)
  - [Secrets](#secrets)
  - [Visibility during Serialization](#visibility-wip)
- [Rate Limiting](#rate-limiting-wip)
- [Injection Prevention](#injection-prevention-wip)

**Other Dev Features**

- [Concurrency](#concurrency)
  - [Lightweight Threads](#lighweight-threads)
  - [LThread Groups](#lthread-groups)
  - [Lifetime jobs](#lifetime-jobs)
- [Many Built-in Functions](#built-in-functions)
- [Easy declaration of CLI Parameters](#declaration-of-cli-parameters--environment-variables)
- [Transactions & Effects (WIP)](#transactions--effects-wip)

[Planned Features](./FUTURE.md)

---

👥 Discord Server: https://discord.gg/53YGx8GzgE

📖 Language Reference: [docs/language-reference.md](docs/language-reference.md)

⚠️ The language is not production ready yet.\
I am working full-time on Inox, please consider donating. Thanks !

## Installation

<!-- **Inox can be used on any operating system by installing the
[VSCode extension](https://marketplace.visualstudio.com/items?itemName=graphr00t.inox).** -->

If you are using Linux, an archive with a binary and some examples is available
in the [release assets](https://github.com/inoxlang/inox/releases)

- uncompress the archive
- install the `inox` binary to `/usr/local/bin`
  ```
  sudo install ./inox -o root -m 0755 /usr/local/bin/inox
  ```

If you want to compile the language yourself go [here](#compile-from-source).

### Editor Support

- [VSCode & VSCodium](https://marketplace.visualstudio.com/items?itemName=graphr00t.inox)
  : LSP, Debug, colorization, snippets

## Learning Inox

You can learn Inox directly in VSCode by creating a file with a name ending with
`.tut.ix`. This is the recommended way.

![tutorial-demo](https://github.com/inoxlang/inox-vscode/raw/master/assets/docs/tutorial-demo.gif)

📖 Language Reference: [docs/language-reference.md](docs/language-reference.md)

<details>
<summary>Scripting</summary>

Inox can be used for scripting & provides a shell. The development of the
language in those domains is not very active because Inox primarily focuses on
Web Application Development.

To learn scripting go [here](./docs/scripting-basics.md). View
[Shell Basics](./docs/shell-basics.md) to learn how to use Inox interactively.

</details>

## Features

### XML expressions

HTML elements can be created without imports using the built-in **html**
namespace and a JSX-like syntax:

```
manifest {}

return html<div> Hello world ! </div>
```

### HTTP Server - Filesystem Routing

Inox comes with a built-in HTTP server that supports filesystem routing:

```
# main.ix
const (
    HOST = https://localhost:8080
)

manifest {
    permissions: {
        provide: HOST
        read: %/...
    }
}

server = http.Server!(HOST, {
    routing: {
        static: /static/
        dynamic: /routes/
    }
})
```

For maximum security, each request is processed in an isolated module:

```
# /routes/api/POST-users.ix

manifest {
    parameters: {
        # JSON body parameter
        name: {
            pattern: %str
        }
    }
    permissions: {
        create: %https://internal-service/users/...
    }
}

username = mod-args.name
...
```

### Built-in Database

Inox includes an embedded database engine. Databases are described in the
manifest at the top of the module:

```
manifest {
    permissions: {
        read: %/...
        write: %/...
    }
    databases: {
        main: {
            resource: ldb://main  #ldb stands for Local Database
            resolution-data: /databases/main/
            expected-schema-update: true
        }
    }
}

# define the pattern for user data
pattern user = {
  name: str
}

dbs.main.update_schema(%{
    users: Set(user, #url)
})
```

Objects can be directly added to and retrieved from the database.\
This is made possible by the fact that most Inox types are constrained to be
[serializable](#serializability).

```
new_user = {name: "John"}
dbs.main.users.add(new_user)

# true
dbs.main.users.has(new_user)
```

You can learn more [here](./docs/language-reference.md#databases).

> The database currently uses a single-file key-value store, it obviously cannot
> handle hundreds of Gigabytes.\
> The improvement of the database engine is a main focus point. The goal is to
> have a DB engine that is aware of the code accessing it (HTTP request
> handlers) in order to smartly pre-fetch and cache data.

### Serializability

Most Inox types (objects, lists, Sets) are serializable so they cannot contain
transient values.

```
object = {
  # error: non-serializable values are not allowed as initial values of properties
  lthread: go do {
    return 1
  }
}

# same error
list = [  
  go do { return 1 }
]
```

The transient counterparts of objects are
[structs](./docs/language-reference.md#structs) (not implemented yet).

```
struct Task {
  name: str
}

task1 = Task{name: "0"}
task2 = Task{name: "1"}

array = Array(task1, task2)
```

### Project Server (LSP)

The Inox binary comes with a **project server** that your IDE connects to.
This server is a LSP server with additional methods, it enables the developer to develop, debug, test, deploy & manage secrets, all from VsCode.

The project server will also provide automatic infrastructure management in the **near future**.

```mermaid
graph LR

subgraph VSCode
  VSCodeVFS(Virtual Filesystem)
end

VSCodeVFS ---> ProjImage
VSCode --->|Invocation & Debug| Runtime(Inox Runtime)
ProjectServer[Project Server]

subgraph ProjectServer
  Runtime
  ProjImage(Project Image)
end

ProjectServer --->|Manages| Infrastructure
ProjectServer --->|Get/Set| Secrets
```

In project mode Inox applications are executed inside a **virtual filesystem** (container) for better
security & reproducibility. Note that this virtual filesystem only exists
in-process, there is no FUSE filesystem and Docker is not used.

**How can I execute binaries if the filesystem only exists inside a process ?**\
You can't, but executing programs compiled to WebAssembly will be soon possible.

### Virtual Filesystems

In project mode Inox applications run inside a **meta filesystem** that persists data on disk.
Files in this filesystem are regular files, metadata and directory structure are stored in a single file named `metadata.kv`.
It's impossible for applications running its filesystem to access an arbitrary file on the disk.

```mermaid
graph LR

subgraph InoxBinary
  Runtime1 --> MetaFS(Meta Filesystem)
  Runtime2 --> InMemFS(In-Memory Filesystem)
  Runtime3 --> OsFS(OS Filesystem)
end

MetaFS -.-> MetadataKV
MetaFS -.-> DFile1
MetaFS -.-> DFile2
OsFS -.-> Disk


subgraph Disk

  subgraph SingleFolder[Single Folder]
    MetadataKV("metadata.kv")
    DFile1("File 01HG3BE...")
    DFile2("File 01HFHVY...")
  end
end
```


### Advanced Testing Engine

**Virtual filesystems**:

```
manifest {}

snapshot = fs.new_snapshot{
    files: :{
        ./file1.txt: "content 1"
        ./dir/: :{
            ./file2.txt: "content 2"
        }
    }
}

testsuite ({
    # a filesystem will be created from the snapshot 
    # for each nested suite and test case.
    fs: snapshot
}) {

    assert fs.exists(/file1.txt)
    fs.rm(/file1.txt)

    testcase {
        # no error
        assert fs.exists(/file1.txt)
        fs.rm(/file1.txt)
    }

    testcase {
        # no error
        assert fs.exists(/file1.txt)
    }
}
```

Inox's testing engine supports executing Inox programs. It automatically creates
a short-lived filesystem from the project's base image.

```
manifest {
    permissions: {
        provide: https://localhost:8080
    }
}

testsuite({
    program: /web-app.ix
}) {
    testcase {
        assert http.exists(https://localhost:8080/)
    }

    testcase {
        assert http.exists(https://localhost:8080/about)
    }
}
```

[Learn More About Testing](./docs/language-reference.md#testing)

### Built-in Browser Automation

```
h = chrome.Handle!()

h.nav https://go.dev/
node = h.html_node!(".Hero-blurb")
h.close()
```

[Documentation](https://github.com/inoxlang/inox/blob/master/docs/builtin.md#browser-automation)

[Examples](https://github.com/inoxlang/inox/tree/master/examples/chrome)

### Permission System

#### **Required Permissions**

Inox features a fine-grained **permission system** that restricts what a module
is allowed to do, here are a few examples of permissions:

- access to the filesystem (read, create, update, write, delete)
- access to the network (several distinct permissions)
  - HTTP (read, create, update, delete, listen)
  - Websocket (read, write, listen)
  - DNS (read)
  - Raw TCP (read, write)
- access to environment variables (read, write, delete)
- create lighweight threads
- execute specific commands

Inox modules always start with a **manifest** that describes the required
permissions.

<img src="./docs/img/fs-malicious-input.png"></img>

<!-- code that appear on the image
manifest {
  permissions: {
    read: %/tmp/...
  }
}

malicious_user_input = /home/
....
print(fs.ls!(malicious_user_input))

-->

Attempting to perform a forbidden operation raises an error:\
`core: error: not allowed, missing permission: [read path(s) /home/]`

#### **Isolation of Dependencies**

In imports the importing module specifies the permissions it **grants** to the
imported module.

`./app.ix`

```
manifest {
  permissions: {
    read: %/...
    create: {threads: {}}
  }
}

import lib ./malicious-lib.ix {
  arguments: {}
  allow: {
    read: %/tmp/...
  }
}
```

`./malicious-lib.ix`

```
manifest {
  permissions: {
    read: %/...
  }
}

data = fs.read!(/etc/passwd)
```

If the imported module asks more permissions than granted an error is thrown:\
`import: some permissions in the imported module's manifest are not granted: [read path(s) /...]`

#### **Process-Level Access Control**

In addition to the checks performed by the permission system, the **inox**
binary uses [Landlock](https://landlock.io/) to restrict file access for the
whole process and its children.

#### **Dropping Permissions**

Sometimes programs have an **initialization** phase, for example a program reads
a file or performs an HTTP request to fetch its configuration. After this phase
it no longer needs some permissions so it can drop them.

```
drop-perms {
  read: %https://**
}
```

### DoS Mitigation

#### **Limits (WIP)**

Limits limit intensive operations, there are three kinds of limits: **byte
rate**, **simple rate** & **total**. They are defined in the manifest and are
[shared](./docs/language-reference.md#limits) with the children of the module.

```
manifest {
    permissions: {
        ...
    }
    limits: {
        "fs/read": 10MB/s
        "http/req": 10x/s
    }
}
```

By default strict limits are applied on HTTP request handlers in order to
mitigate some types of DoS.

[Learn More](./docs/language-reference.md#limits)

### Sensitive Data Protection

#### **Secrets**

Secrets are special Inox values, they can only be created by defining an
**environment variable** with a pattern like %secret-string or by storing a
[project secret](./docs/project.md#project-secrets).

- The content of the secret is **hidden** when printed or logged.
- Secrets are not serializable so they cannot be included in HTTP responses.
- A comparison involving a secret always returns **false**.

```
manifest {
    ...
    env: %{
        API_KEY: %secret-string
    }
    ...
}

API_KEY = env.initial.API_KEY
```

#### **Visibility (WIP)**

_This feature is **very much** work in progress._

[**Excessive Data Exposure**](https://apisecurity.io/encyclopedia/content/owasp/api3-excessive-data-exposure.htm)
occurs when an HTTP API returns more data than needed, potentially exposing
sensitive information.\
In order to mitigate this type of vunerability the serialization of Inox values
involves the concepts of **value visibility** and **property visibility**.

Let's take an example, here is an Inox object:

```
{
  non_sensitive: 1, 
  x: EmailAddress"example@mail.com"
  age: 30, 
  passwordHash: "x"
}
```

The serialization of the object will not include properties having a **sensitive
name** or a **sensitive value**:

```
{
  "non_sensitive": 1
}
```

The visibility of properties can be configured using the `_visibility_`
metaproperty.

```
{
  _visibility_ {
    {
      public: .{passwordHash}
    }
  }
  passwordHash: "x"
}
```

ℹ️ In the near future the visibility will be configurable directly in patterns &
database schemas.

### Rate Limiting (WIP)

Inox's HTTP Server has an embedded rate limiting engine. It is pretty basic at
the moment, but it will make decisions based on logs & access patterns in the
near future.

### Injection Prevention (WIP)

In Inox interpolations are always restricted in order to prevent **injections**
and regular strings are **never trusted**. URLs & paths are first-class values
and must be used to perform network or filesystem operations.

#### **URL Interpolations**

When you dynamically create URLs the interpolations are restricted based on
their location (path, query).

```
https://example.com/{path}?a={param}
```

In short, most malicious `path` and `param` values provided by a malevolent user
will cause an error at runtime.

<details>
<summary>
 Click for more explanations.
</summary>

Let's say that you are writing a piece of code that fetches **public** data from
a private/internal service and returns the result to a user. You are using the
query parameter `?admin=false` in the URL because only public data should be
returned.

```
public_data = http.read!(https://private-service{path}?admin=false)
```

The way in which the user interacts with your code is not important here, let's
assume that the user can send any value for `path`. Obviously this is a very bad
idea from a security standpoint. A malicious path could be used to:

- perform a directory traversal if the private service has a vulnerable endpoint
- inject a query parameter `?admin=true` to retrieve private data
- inject a port number

In Inox the URL interpolations are special, based on the location of the
interpolation specific checks are performed:

```
https://example.com/api/{path}/?x={x}
```

- interpolations before the `'?'` are **path** interpolations
  - the strings/characters `'..'`, `'\\'`, `'?'` and `'#'` are forbidden
  - the URL encoded versions of `'..'` and `'\\'` are forbidden
  - `':'` is forbidden at the start of the finalized path (after all
    interpolations have been evaluated)
- interpolations after the `'?'` are **query** interpolations
  - the characters `'&'` and `'#'` are forbidden

In the example if the path `/data?admin=true` is received the Inox runtime will
throw an error:

```
URL expression: result of a path interpolation should not contain any of the following substrings: "..", "\" , "*", "?"
```

</details>

### Concurrency

#### **Lighweight threads**

```
lthread = go {} do {
  print("hello from goroutine !")
  return 1
}

# 1
result = lthread.wait_result!()
```

#### **Lthread Groups**

```
group = LThreadGroup()
lthread1 = go {group: group} do read!(https://jsonplaceholder.typicode.com/posts/1)
lthread2 = go {group: group} do read!(https://jsonplaceholder.typicode.com/posts/2)

results = group.wait_results!()
```

#### **Lifetime Jobs**

Lifetime jobs are lthreads linked to an object.

```
object = {
  lifetimejob #handle-messages {
    for msg in watch_received_messages(self){
      # handle messages
    }
  }
}
```

### Built-in Functions

Inox comes with many built-in functions for:

- browser automation
- file manipulation
- HTTP resource manipulation
- data container constructors (Graph, Tree, ...)

**[List of Built-in Functions](./docs/builtin.md)**

### Declaration of CLI Parameters & Environment Variables

CLI parameters & environment variables can be described in the manifest:

```
manifest {
    parameters: {
        # positional parameters are listed at the start
        {
            name: #dir
            pattern: %path
            rest: false
            description: "root directory of the project"
        }
        # non positional parameters
        clean-existing: {
            pattern: %bool
            default: false
            description: "if true delete <dir> if it already exists"
        }
    }
    env: {
      API_KEY: %secret-string
    }

    permissions: {
        write: IWD_PREFIX # initial working directory
        delete: IWD_PREFIX
    }
}

# {
#   "dir": ...
#   "clean-existing": ...
# }
args = mod-args

API_KEY = env.initial.API_KEY
```

#### Help Message Generation

```
$ inox run test.ix 
not enough CLI arguments
usage: <dir path> [--clean-existing]

required:

  dir: %path
      root directory of the project

options:

  clean-existing (--clean-existing): boolean
      if true delete <dir> if it already exists
```

### Transactions & Effects (WIP)

Inox allows you to attach a **transaction** to the current execution context
(think SQL transactions). When a **side effect** happens it is recorded in the
transaction. If the execution is cancelled for whatever reason the transaction
is automatically **rollbacked** and reversible effects are reversed. Some
effects such as database changes are only applied when the transaction is
committed.

```
tx = start_tx()

# effect
fs.mkfile ./file.txt 

# rollback transaction --> delete ./file.txt
cancel_exec()
```

ℹ️ A transaction is created for each
[HTTP request](#http-server---filesystem-routing).

## Compile from Source

- clone this repository
- `cd` into the directory
- run `go build cmd/inox/inox.go`

## Early Sponsors

<table>
  <tr>
   <td align="center"><a href="https://github.com/Lexterl33t"><img src="https://avatars.githubusercontent.com/u/44911576?v=4&s=120" width="120" alt="Lexter"/><br />Lexter</a></td>
   <td align="center"><a href="https://github.com/datamixio"><img src="https://avatars.githubusercontent.com/u/8696011?v=4&s=120"
   width="120" alt="Datamix.io"/><br />Datamix.io</a></td>
  </tr>
</table>

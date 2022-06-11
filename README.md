# Cookie

Cookie is a simple Text Editor _forked (more like copy and pasted)_ from [mini](https://github.com/hibiken/mini) text editor.

## Features

Has all the features of mini text editor.

- Syntax highlighting
- Searching

What's more? There is better syntax highlighting than mini text editor. with an editable config file where you can change your color palette, tab stop, and more.

You can also add more syntax highlighting languages that come preinstalled with Cookie, but you can also add your own.

All the config files can be found at the following directory:

```txt
$HOME/.config/cookie/
```

## Installation

```txt
go install github.com/cookie-for-pres/cookie@v0.1.4
```

## Usage

```txt
cookie <filename>
```

## Key bindings

```txt
Ctrl-Q: quit
Ctrl-S: save
Ctrl-F: find
Ctrl-D: delete line
```

## License

Cookie editor is released under MIT license. See [LICENSE](https://github.com/cookie-for-pres/cookie/blob/main/LICENSE).

## Other

256 ascii color palette [here](https://robotmoon.com/256-colors/).

### Default Config File

```json
{
  "tab_stop": 4,
  "quit_times": 1,
  "empty_line_char": "~",
  "color_palette": {
    "normal": 15,
    "comment": 238,
    "multiline_comment": 238,
    "keyword1": 99,
    "keyword2": 141,
    "string": 14,
    "number": 147,
    "boolean": 6,
    "match": 32
  }
}
```

### Default Syntax Config File

```json
[
  {
    "filetype": "c",
    "filematch": [".c", ".h", "cpp", ".cc"],
    "keywords": [
      "switch",
      "if",
      "while",
      "for",
      "break",
      "continue",
      "return",
      "else",
      "struct",
      "union",
      "typedef",
      "static",
      "enum",
      "class",
      "case",

      "int|",
      "long|",
      "double|",
      "float|",
      "char|",
      "unsigned|",
      "signed|",
      "void|"
    ],
    "scs": "//",
    "mcs": "/*",
    "mce": "*/",
    "flags": {
      "highlight_numbers": true,
      "highlight_strings": true,
      "highlight_booleans": true
    }
  },
  {
    "filetype": "go",
    "filematch": [".go"],
    "keywords": [
      "break",
      "default",
      "func",
      "interface",
      "select",
      "case",
      "defer",
      "go",
      "map",
      "struct",
      "chan",
      "else",
      "goto",
      "package",
      "switch",
      "const",
      "fallthrough",
      "if",
      "range",
      "type",
      "continue",
      "for",
      "import",
      "return",
      "var",

      "append|",
      "bool|",
      "byte|",
      "cap|",
      "close|",
      "complex|",
      "complex64|",
      "complex128|",
      "error|",
      "uint16|",
      "copy|",
      "false|",
      "float32|",
      "float64|",
      "imag|",
      "int|",
      "int8|",
      "int16|",
      "uint32|",
      "int32|",
      "int64|",
      "iota|",
      "len|",
      "make|",
      "new|",
      "nil|",
      "panic|",
      "uint64|",
      "print|",
      "println|",
      "real|",
      "recover|",
      "rune|",
      "string|",
      "true|",
      "uint|",
      "uint8|",
      "uintptr|",
      "delete|",
      "error|",
      "float32|",
      "float64|"
    ],
    "scs": "//",
    "mcs": "/*",
    "mce": "*/",
    "flags": {
      "highlight_numbers": true,
      "highlight_strings": true,
      "highlight_booleans": true
    }
  },
  {
    "filetype": "python",
    "filematch": [".py"],
    "keywords": [
      "and",
      "as",
      "assert",
      "break",
      "class",
      "continue",
      "def",
      "del",
      "elif",
      "else",
      "except",
      "exec",
      "finally",
      "for",
      "if",
      "in",
      "is",
      "lambda",
      "not",
      "or",
      "pass",

      "raise|",
      "return|",
      "try|",
      "while|",
      "with|",
      "yield|",
      "global|",
      "import|",
      "from|",
      "input|",
      "print|",
      "eval|"
    ],
    "scs": "#",
    "mcs": "#",
    "mce": "#",
    "flags": {
      "highlight_numbers": true,
      "highlight_strings": true,
      "highlight_booleans": true
    }
  },
  {
    "filetype": "java",
    "filematch": [".java"],
    "keywords": [
      "abstract",
      "continue",
      "for",
      "new",
      "switch",
      "assert",
      "default",
      "goto",
      "package",
      "synchronized",
      "boolean",
      "do",
      "if",
      "private",
      "this",
      "break",
      "double",
      "implements",
      "protected",
      "throw",
      "byte",
      "else",
      "import",
      "public",
      "throws",
      "case",
      "enum",
      "instanceof",
      "return",
      "transient",
      "catch",
      "extends",
      "int",
      "short",
      "try",
      "char",
      "final",
      "interface",
      "static",
      "void",

      "int|",
      "long|",
      "double|",
      "float|",
      "char|",
      "unsigned|",
      "signed|",
      "void|"
    ],
    "scs": "//",
    "mcs": "/*",
    "mce": "*/",
    "flags": {
      "highlight_numbers": true,
      "highlight_strings": true,
      "highlight_booleans": true
    }
  },
  {
    "filetype": "javascript",
    "filematch": [".js", ".ts"],
    "keywords": [
      "break",
      "case",
      "catch",
      "class",
      "const",
      "continue",
      "debugger",
      "default",
      "delete",
      "do",
      "else",
      "enum",
      "export",
      "extends",
      "false",
      "finally",
      "for",
      "function",
      "if",
      "implements",
      "import",
      "in",
      "instanceof",
      "interface",
      "let",
      "new",
      "null",
      "package",
      "private",
      "protected",
      "public",
      "return",
      "static",
      "super",
      "switch",
      "this",
      "throw",
      "true",
      "try",
      "typeof",
      "var",
      "void",
      "while",
      "with",
      "yield",

      "int|",
      "long|",
      "double|",
      "float|",
      "char|",
      "unsigned|",
      "signed|",
      "void|"
    ],
    "scs": "//",
    "mcs": "/*",
    "mce": "*/",
    "flags": {
      "highlight_numbers": true,
      "highlight_strings": true,
      "highlight_booleans": true
    }
  },
  {
    "filetype": "rust",
    "filematch": [".rs"],
    "keywords": [
      "break",
      "case",
      "catch",
      "class",
      "const",
      "continue",
      "debugger",
      "default",
      "delete",
      "do",
      "else",
      "enum",
      "export",
      "extends",
      "false",
      "finally",
      "for",
      "function",
      "if",
      "implements",
      "import",
      "in",
      "instanceof",
      "interface",
      "let",
      "new",
      "null",
      "package",
      "private",
      "protected",
      "public",
      "return",
      "static",
      "super",
      "switch",
      "this",
      "throw",
      "true",
      "try",
      "typeof",
      "var",
      "void",
      "while",
      "with",
      "yield",

      "int|",
      "long|",
      "double|",
      "float|",
      "char|",
      "unsigned|",
      "signed|",
      "void|"
    ],
    "scs": "//",
    "mcs": "/*",
    "mce": "*/",
    "flags": {
      "highlight_numbers": true,
      "highlight_strings": true,
      "highlight_booleans": true
    }
  }
]
```

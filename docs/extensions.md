<!-- Generated with Stardoc: http://skydoc.bazel.build -->

Module extensions of hdlf_rules_cam.

```starlark
freecad = use_extension("@hdlf_rules_cam//cam:extensions.bzl", "freecad")
freecad.toolchain(version = "1.1.4")
```

<a id="freecad"></a>

## freecad

<pre>
freecad = use_extension("@hdlf_rules_cam//cam:extensions.bzl", "freecad")
freecad.toolchain(<a href="#freecad.toolchain-version">version</a>)
</pre>

The prebuilt FreeCAD toolchain.

hdlf_rules_cam registers it, so a user needs nothing to get FreeCAD
1.1.4. The tree is the official AppImage, unpacked and
pruned to `freecadcmd` and the CAM workbench (about 754 MB unpacked); it runs with no
display and no host libraries but glibc.

The root module can pick another version that hdlf_rules_cam has a
prebuilt for:

```starlark
freecad = use_extension("@hdlf_rules_cam//cam:extensions.bzl", "freecad")
freecad.toolchain(version = "1.1.4")
```


**TAG CLASSES**

<a id="freecad.toolchain"></a>

### toolchain

Picks the FreeCAD version. Only the root module's tag counts.

**Attributes**

| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="freecad.toolchain-version"></a>version |  A FreeCAD version that hdlf_rules_cam has a prebuilt for.   | String | optional |  `"1.1.4"`  |



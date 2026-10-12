rootProject.name = "rosa-java"

include(
    "rosa-domain",
    "rosa-application",
    "rosa-persistence",
    "rosa-compatibility",
    "rosa-platform-windows",
    "rosa-reports",
    "rosa-ui",
    "rosa-launcher",
    "rosa-architecture-tests",
)

rootProject.children.forEach { project ->
    project.projectDir = file("modules/${project.name}")
}


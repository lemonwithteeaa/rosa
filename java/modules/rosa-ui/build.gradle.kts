val javafxPlatform = when {
    System.getProperty("os.name").startsWith("Windows") -> "win"
    System.getProperty("os.name").startsWith("Mac") -> "mac"
    else -> "linux"
}

dependencies {
    api(project(":rosa-application"))
    api("org.openjfx:javafx-base:${libs.versions.javafx.get()}:$javafxPlatform")
    api("org.openjfx:javafx-graphics:${libs.versions.javafx.get()}:$javafxPlatform")
    api("org.openjfx:javafx-controls:${libs.versions.javafx.get()}:$javafxPlatform")
}


plugins {
    application
}

dependencies {
    implementation(project(":rosa-application"))
    implementation(project(":rosa-compatibility"))
    implementation(project(":rosa-persistence"))
    implementation(project(":rosa-platform-windows"))
    implementation(project(":rosa-reports"))
    implementation(project(":rosa-ui"))
}

application {
    mainClass = "ru.rosa.launcher.RosaLauncher"
    applicationDefaultJvmArgs = listOf("-Dfile.encoding=UTF-8")
}


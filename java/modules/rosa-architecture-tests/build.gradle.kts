dependencies {
    testImplementation(project(":rosa-domain"))
    testImplementation(project(":rosa-application"))
    testImplementation(project(":rosa-persistence"))
    testImplementation(project(":rosa-compatibility"))
    testImplementation(project(":rosa-platform-windows"))
    testImplementation(project(":rosa-reports"))
    testImplementation(project(":rosa-ui"))
    testImplementation(project(":rosa-launcher"))
    testImplementation(libs.archunit.junit5)
}


package ru.rosa.architecture;

import static com.tngtech.archunit.lang.syntax.ArchRuleDefinition.noClasses;

import com.tngtech.archunit.junit.AnalyzeClasses;
import com.tngtech.archunit.junit.ArchTest;
import com.tngtech.archunit.lang.ArchRule;

@AnalyzeClasses(packages = "ru.rosa")
class DependencyRulesTest {
    private static final String[] OUTSIDE_DOMAIN = {
        "ru.rosa.application..",
        "ru.rosa.persistence..",
        "ru.rosa.compatibility..",
        "ru.rosa.platform..",
        "ru.rosa.reports..",
        "ru.rosa.ui..",
        "ru.rosa.launcher.."
    };

    private static final String[] OUTSIDE_APPLICATION = {
        "ru.rosa.persistence..",
        "ru.rosa.compatibility..",
        "ru.rosa.platform..",
        "ru.rosa.reports..",
        "ru.rosa.ui..",
        "ru.rosa.launcher.."
    };

    @ArchTest
    static final ArchRule domain_is_independent = noClasses()
            .that().resideInAPackage("ru.rosa.domain..")
            .should().dependOnClassesThat().resideInAnyPackage(OUTSIDE_DOMAIN);

    @ArchTest
    static final ArchRule application_only_depends_inward = noClasses()
            .that().resideInAPackage("ru.rosa.application..")
            .should().dependOnClassesThat().resideInAnyPackage(OUTSIDE_APPLICATION);

    @ArchTest
    static final ArchRule ui_does_not_know_adapters = noClasses()
            .that().resideInAPackage("ru.rosa.ui..")
            .should().dependOnClassesThat().resideInAnyPackage(
                    "ru.rosa.persistence..",
                    "ru.rosa.compatibility..",
                    "ru.rosa.platform..",
                    "ru.rosa.reports..",
                    "ru.rosa.launcher..");
}

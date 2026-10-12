package ru.rosa.compatibility;

import java.io.IOException;
import java.io.InputStream;
import java.io.InputStreamReader;
import java.nio.charset.StandardCharsets;
import java.util.Objects;
import java.util.Properties;
import ru.rosa.application.port.GoBaselinePort;
import ru.rosa.domain.model.MigrationStage;
import ru.rosa.domain.model.ProductIdentity;

/** Reads the migration oracle descriptor bundled with the Java application. */
public final class ClasspathGoBaselineAdapter implements GoBaselinePort {
    static final String RESOURCE_NAME = "rosa-baseline.properties";

    private final ClassLoader classLoader;

    public ClasspathGoBaselineAdapter() {
        this(ClasspathGoBaselineAdapter.class.getClassLoader());
    }

    ClasspathGoBaselineAdapter(ClassLoader classLoader) {
        this.classLoader = Objects.requireNonNull(classLoader, "classLoader");
    }

    @Override
    public ProductIdentity load() {
        Properties properties = new Properties();
        try (InputStream input = classLoader.getResourceAsStream(RESOURCE_NAME)) {
            if (input == null) {
                throw new BaselineConfigurationException("Missing " + RESOURCE_NAME);
            }
            properties.load(new InputStreamReader(input, StandardCharsets.UTF_8));
        } catch (IOException exception) {
            throw new BaselineConfigurationException("Cannot read " + RESOURCE_NAME, exception);
        }

        try {
            int stageNumber = Integer.parseInt(required(properties, "migration.stage.number"));
            return new ProductIdentity(
                    required(properties, "product.name"),
                    required(properties, "go.baseline.version"),
                    new MigrationStage(stageNumber, required(properties, "migration.stage.name")));
        } catch (IllegalArgumentException exception) {
            throw new BaselineConfigurationException("Invalid " + RESOURCE_NAME, exception);
        }
    }

    private static String required(Properties properties, String key) {
        String value = properties.getProperty(key);
        if (value == null || value.isBlank()) {
            throw new IllegalArgumentException("Missing property " + key);
        }
        return value;
    }
}


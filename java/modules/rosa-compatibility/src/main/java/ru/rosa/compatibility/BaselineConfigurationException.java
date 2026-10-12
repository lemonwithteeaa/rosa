package ru.rosa.compatibility;

public final class BaselineConfigurationException extends RuntimeException {
    private static final long serialVersionUID = 1L;

    public BaselineConfigurationException(String message) {
        super(message);
    }

    public BaselineConfigurationException(String message, Throwable cause) {
        super(message, cause);
    }
}


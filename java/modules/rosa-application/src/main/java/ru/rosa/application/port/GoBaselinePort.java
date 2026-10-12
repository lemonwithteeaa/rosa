package ru.rosa.application.port;

import ru.rosa.domain.model.ProductIdentity;

/** Provides the immutable identity of the Go version used as migration oracle. */
@FunctionalInterface
public interface GoBaselinePort {
    ProductIdentity load();
}


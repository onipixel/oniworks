package app

// ServiceProvider is the interface all service providers must implement.
// Providers bootstrap framework features into the application container.
type ServiceProvider interface {
	// Register binds services into the container. No services should be resolved here
	// because other providers may not have registered their services yet.
	Register(app *Application)

	// Boot is called after all providers have been registered. Safe to resolve services.
	Boot(app *Application)
}

// DeferrableProvider is an optional interface providers can implement to signal that
// they only need to be registered when one of their provided services is requested.
type DeferrableProvider interface {
	ServiceProvider
	Provides() []string
}

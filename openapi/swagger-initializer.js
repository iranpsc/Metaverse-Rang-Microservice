// Swagger UI initializer — works behind Kong (/docs) and direct (:8081).
window.onload = function () {
  const prefix = window.location.pathname.startsWith("/docs") ? "/docs" : "";

  window.ui = SwaggerUIBundle({
    url: prefix + "/openapi/openapi.yaml",
    dom_id: "#swagger-ui",
    deepLinking: true,
    presets: [SwaggerUIBundle.presets.apis, SwaggerUIStandalonePreset],
    plugins: [SwaggerUIBundle.plugins.DownloadUrl],
    layout: "StandaloneLayout",
    persistAuthorization: true,
    tryItOutEnabled: true,
    validatorUrl: null,
  });
};

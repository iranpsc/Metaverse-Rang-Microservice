const Sentry = require('@sentry/node');

let enabled = false;

function initSentry(serviceName, app) {
  const dsn = process.env.SENTRY_DSN;
  if (!dsn) {
    return false;
  }

  const parsedSampleRate = parseFloat(process.env.SENTRY_TRACES_SAMPLE_RATE || '0');
  const tracesSampleRate = Number.isNaN(parsedSampleRate) ? 0 : parsedSampleRate;

  const integrations = [Sentry.httpIntegration()];
  if (app) {
    integrations.push(Sentry.expressIntegration({ app }));
  }

  Sentry.init({
    dsn,
    environment: process.env.SENTRY_ENVIRONMENT || process.env.NODE_ENV || 'development',
    release: process.env.SENTRY_RELEASE || undefined,
    tracesSampleRate,
    serverName: serviceName,
    integrations,
  });

  enabled = true;
  return true;
}

function captureException(error) {
  if (enabled && error) {
    Sentry.captureException(error);
  }
}

async function flush(timeoutMs = 2000) {
  if (!enabled) {
    return;
  }

  await Sentry.close(timeoutMs);
}

module.exports = {
  Sentry,
  initSentry,
  captureException,
  flush,
  enabled: () => enabled,
};

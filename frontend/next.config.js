/** @type {import('next').NextConfig} */
const nextConfig = {
    reactStrictMode: true,
    webpack: (config, { isServer }) => {
        // Ensure encryption module is only loaded on client side
        if (!isServer) {
            config.resolve.fallback = {
                ...config.resolve.fallback,
                crypto: false,
            };
        }
        return config;
    },
};

module.exports = nextConfig;

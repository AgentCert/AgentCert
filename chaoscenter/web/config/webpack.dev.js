const path = require('path');
const fs = require('fs');
const { merge } = require('webpack-merge');
const MiniCssExtractPlugin = require('mini-css-extract-plugin');
const { DefinePlugin } = require('webpack');
const commonConfig = require('./webpack.common');

require('dotenv').config();

const CONTEXT = process.cwd();
const baseUrl = process.env.BASE_URL;
const targetLocalHost = (process.env.TARGET_LOCALHOST && JSON.parse(process.env.TARGET_LOCALHOST)) ?? true; // set to false to target baseUrl environment instead of localhost
const frontendPort = Number.parseInt(process.env.FRONTEND_PORT || '', 10) || 2001;
const graphQLProxyPort = Number.parseInt(process.env.GQL_PROXY_PORT || '', 10) || 8080;
const authProxyPort = Number.parseInt(process.env.AUTH_PROXY_PORT || '', 10) || 3030;

const certificateExists = fs.existsSync(path.join(CONTEXT, 'certificates/localhost.pem'));

// certificates are required in non CI environments only
if (!certificateExists) {
  throw new Error('The certificate is missing, please run `yarn generate-certificate`');
}

const devConfig = {
  mode: 'development',
  devtool: 'cheap-module-source-map',
  cache: { type: 'filesystem' },
  output: {
    filename: '[name].js',
    chunkFilename: '[name].[id].js'
  },
  devServer: {
    static: [path.join(process.cwd(), 'src/static')],
    historyApiFallback: true,
    port: frontendPort,
    server: {
      type: 'https',
      options: {
        key: fs.readFileSync(path.resolve(CONTEXT, 'certificates/localhost-key.pem')),
        cert: fs.readFileSync(path.resolve(CONTEXT, 'certificates/localhost.pem'))
      }
    },
    proxy: {
      '/api': {
        pathRewrite: { '^/api': '' },
        target: process.env.CHAOS_MANAGER
          ? process.env.CHAOS_MANAGER
          : targetLocalHost
          ? `http://localhost:${graphQLProxyPort}`
          : `${baseUrl}/api`,
        secure: false,
        changeOrigin: true,
        logLevel: 'info'
      },
      '/auth': {
        pathRewrite: { '^/auth': '' },
        target: process.env.CHAOS_MANAGER
          ? process.env.CHAOS_MANAGER
          : targetLocalHost
<<<<<<< Updated upstream
          ? 'http://localhost:3030'
=======
          ? `http://localhost:${authProxyPort}`
>>>>>>> Stashed changes
          : `${baseUrl}/auth`,
        secure: false,
        changeOrigin: true,
        logLevel: 'info'
      }
    }
  },
  plugins: [
    new MiniCssExtractPlugin({
      filename: '[name].css',
      chunkFilename: '[name].[id].css',
      ignoreOrder: true
    }),
    new DefinePlugin({
      'process.env': '{}', // required for @blueprintjs/core
      __DEV__: true
    })
  ]
};

module.exports = merge(commonConfig, devConfig);

const { merge } = require('webpack-merge');
const { DefinePlugin } = require('webpack');
const MiniCssExtractPlugin = require('mini-css-extract-plugin');
const CircularDependencyPlugin = require('circular-dependency-plugin');

const commonConfig = require('./webpack.common');

require('dotenv').config();

const agentCertApiBaseUrl = process.env.AGENTCERT_API_BASE_URL || '';

const prodConfig = {
  mode: 'production',
  devtool: 'hidden-source-map',
  output: {
    filename: '[name].[contenthash:6].js',
    chunkFilename: '[name].[id].[contenthash:6].js'
  },
  plugins: [
    new MiniCssExtractPlugin({
      filename: '[name].[contenthash:6].css',
      chunkFilename: '[name].[id].[contenthash:6].css',
      ignoreOrder: true
    }),
    new CircularDependencyPlugin({
      exclude: /node_modules/,
      failOnError: true
    }),
    new DefinePlugin({
      __AGENTCERT_API_BASE_URL__: JSON.stringify(agentCertApiBaseUrl)
    })
  ]
};

module.exports = merge(commonConfig, prodConfig);

"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const pkg_root_webpack_plugin_origin_1 = __importDefault(require("@coze-arch/pkg-root-webpack-plugin-origin"));
const utils_1 = require("./utils");
class PkgRootWebpackPlugin extends pkg_root_webpack_plugin_origin_1.default {
    constructor(options) {
        const rushJson = (0, utils_1.getRushConfiguration)();
        const rushJsonPackagesDir = rushJson.projects.map(item => item.projectFolder);
        // .filter(item => !item.includes('/apps/'));
        const mergedOptions = Object.assign({}, options || {}, {
            root: '@',
            packagesDirs: rushJsonPackagesDir,
            // 排除apps/*，减少处理时间
            excludeFolders: [],
        });
        super(mergedOptions);
    }
}
exports.default = PkgRootWebpackPlugin;

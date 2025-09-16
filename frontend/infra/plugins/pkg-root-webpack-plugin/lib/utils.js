"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.getRushConfiguration = void 0;
const rush_sdk_1 = require("@rushstack/rush-sdk");
exports.getRushConfiguration = (() => {
    let rushConfig;
    return () => {
        if (!rushConfig) {
            rushConfig = rush_sdk_1.RushConfiguration.loadFromDefaultLocation({});
        }
        return rushConfig;
    };
})();

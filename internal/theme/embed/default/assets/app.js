// Plainship 默认主题脚本.
// 第一阶段保持最小, 未来可扩展交互能力.
document.addEventListener("DOMContentLoaded", function () {
    var year = new Date().getFullYear();
    var el = document.getElementById("copyright-year");
    if (el) {
        el.textContent = year;
    }
});

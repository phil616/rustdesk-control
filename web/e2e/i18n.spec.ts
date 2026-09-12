import { test, expect } from "@playwright/test";

test("Chinese preference, errors, forms and language switching across pages", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await page.goto("/login");
  await page
    .getByRole("combobox", { name: "Language / 语言" })
    .press("ArrowDown");
  await page.getByTitle("简体中文", { exact: true }).click();
  await expect(page.locator("html")).toHaveAttribute("lang", "zh-CN");
  await page.getByLabel("管理员密码").fill("wrong-password");
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await expect(page.getByRole("alert")).toContainText("密码错误");
  await page.reload();
  await expect(page.getByLabel("管理员密码")).toBeVisible();
  await page.getByLabel("管理员密码").fill("browser-test-password");
  await page.getByRole("button", { name: "登录", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "概览", exact: true }),
  ).toBeVisible();
  await expect(page.getByText("设备总数", { exact: true })).toBeVisible();
  await page.getByRole("link", { name: "设备", exact: true }).click();
  await expect(page.getByPlaceholder("搜索 ID 或设备名称")).toBeVisible();
  await page.getByRole("link", { name: "详情", exact: true }).click();
  await expect(page.getByText("设备 UUID", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "重置身份", exact: true }).click();
  await expect(page.getByText("确认重置身份？", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "取消", exact: true }).click();
  await page.getByRole("link", { name: "设置", exact: true }).click();
  await page
    .getByLabel("中继服务器", { exact: true })
    .fill("unsaved.example.com");
  await page
    .getByRole("combobox", { name: "Language / 语言" })
    .press("ArrowDown");
  await page.getByTitle("English", { exact: true }).click();
  await expect(page.getByLabel("Relay Server", { exact: true })).toHaveValue(
    "unsaved.example.com",
  );
  await expect(page.locator("html")).toHaveAttribute("lang", "en-US");
  await page
    .getByRole("combobox", { name: "Language / 语言" })
    .press("ArrowDown");
  await page.getByTitle("简体中文", { exact: true }).click();
  await page.getByRole("button", { name: "修改密码并结束所有会话" }).click();
  await expect(page.getByText("请输入当前密码")).toBeVisible();
  await page.getByRole("link", { name: "审计", exact: true }).click();
  await expect(page.getByRole("heading", { name: "审计日志" })).toBeVisible();
  await expect(
    page.getByRole("cell", { name: "管理员登录", exact: true }).first(),
  ).toBeVisible();
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.locator(".ant-layout-sider")).toHaveCSS("width", "0px");
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({
    path: "../docs/validation/chinese-mobile.png",
    fullPage: true,
  });
  await page.reload();
  await expect(page.getByRole("heading", { name: "审计日志" })).toBeVisible();
  expect(await page.evaluate(() => ({ ...localStorage }))).toEqual({
    "rdc.language": "zh-CN",
  });
  expect(await page.evaluate(() => sessionStorage.length)).toBe(0);
  expect(errors).toEqual([]);
});

test("Chinese browser locale is detected and switching works when storage is blocked", async ({
  browser,
  baseURL,
}) => {
  const context = await browser.newContext({
    locale: "zh-CN",
    ignoreHTTPSErrors: true,
  });
  const page = await context.newPage();
  await page.addInitScript(() => {
    Object.defineProperty(window, "localStorage", {
      get() {
        throw new DOMException("Blocked", "SecurityError");
      },
    });
  });
  await page.goto(baseURL + "/login");
  await expect(page.getByLabel("管理员密码")).toBeVisible();
  await page
    .getByRole("combobox", { name: "Language / 语言" })
    .press("ArrowDown");
  await page.getByTitle("English", { exact: true }).click();
  await expect(page.getByLabel("Administrator password")).toBeVisible();
  await context.close();
});

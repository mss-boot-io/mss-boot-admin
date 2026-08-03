import { chromium } from '@playwright/test';

(async () => {
  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage();
  await page.goto('http://localhost:8000/user/login');
  await page.waitForLoadState('networkidle');
  await page.waitForTimeout(2000);
  
  const html = await page.evaluate(() => {
    const inputs = document.querySelectorAll('.ant-input');
    const results = [];
    inputs.forEach((input, i) => {
      const parent = input.parentElement;
      const grandparent = parent?.parentElement;
      results.push({
        index: i,
        inputClass: input.className,
        inputId: input.id,
        inputName: input.getAttribute('name'),
        inputType: input.getAttribute('type'),
        parentClass: parent?.className,
        grandparentClass: grandparent?.className,
        outerHTML: input.outerHTML.slice(0, 300)
      });
    });
    return results;
  });
  
  console.log(JSON.stringify(html, null, 2));
  
  await browser.close();
})();

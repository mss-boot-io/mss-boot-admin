import { Request, Response } from 'express';

const waitTime = (time: number = 100) => {
  return new Promise((resolve) => {
    setTimeout(() => {
      resolve(true);
    }, time);
  });
};

const { ANT_DESIGN_PRO_ONLY_DO_NOT_USE_IN_YOUR_PRODUCTION } = process.env;

/**
 * 当前用户的权限，如果为空代表没登录
 * current user access， if is '', user need login
 * 如果是 pro 的预览，默认是有权限的
 */
let access = ANT_DESIGN_PRO_ONLY_DO_NOT_USE_IN_YOUR_PRODUCTION === 'site' ? 'admin' : '';

const getAccess = () => {
  return access;
};

export default {
  'GET /api/roles': (req: Request, res: Response) => {
    if (!getAccess()) {
      res.status(401).send({
        data: {
          isLogin: false,
        },
        errorCode: '401',
        errorMessage: '请先登录！',
        success: true,
      });
      return;
    }
    res.send({
      success: true,
      data: [
        {
          id: 'a',
          name: '管理员',
          description: '管理员',
          status: 1,
          creatorId: 1,
          createTime: '2020-12-12 12:12:12',
          deleted: 0,
        },
        {
          id: 'b',
          name: '普通用户',
          description: '普通用户',
          status: 1,
          creatorId: 1,
          createTime: '2020-12-12 12:12:12',
          deleted: 0,
        },
      ],
      total: 2,
      pageSize: 10,
      current: 1,
    });
  },
};

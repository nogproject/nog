import { createErrorModule } from './throw.js';

const platform = {
  where: 'client',
  errorLog: null,
};

const NogError = createErrorModule({ platform });
const { nogthrow } = NogError;

export {
  createErrorModule,
  nogthrow,
};

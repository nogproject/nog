import fs from 'fs';
import { WebApp } from 'meteor/webapp';

function initUnixSocketMode() {
  const socketPath = process.env.UNIX_SOCKET_PATH;
  const socketMode = process.env.UNIX_SOCKET_MODE;
  if (!(socketPath && socketMode)) {
    return;
  }

  const mode = parseInt(socketMode, 8);
  if (Number.isNaN(mode)) {
    console.error('[nog-app-2] Invalid UNIX_SOCKET_MODE.');
    return;
  }

  WebApp.onListening(() => {
    fs.chmod(socketPath, mode, (err) => {
      if (err) {
        console.error(
          '[nog-app-2] Failed to change mode of Unix domain socket.',
          'path', socketPath,
          'err', err,
        );
        return;
      }
      console.log(
        '[nog-app-2] Changed Unix domain socket mode.',
        'path', socketPath,
        'mode', socketMode,
      );
    });
  });
}

export {
  initUnixSocketMode,
};
